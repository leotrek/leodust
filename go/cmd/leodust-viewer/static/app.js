const EARTH_RADIUS_METERS = 6_378_137;
const DEG_TO_RAD = Math.PI / 180;
const EARTH_TEXTURE_MIN_SIZE = 220;
const EARTH_TEXTURE_MAX_SIZE = 520;
const EARTH_TEXTURE_CACHE = new Map();
const VIEWER_CONFIG_PATH = "./viewer-config.json";
const DEFAULT_VIEWER_CONFIG = {
  EarthImagePath: "./earth/World_Map_Blank.svg",
  SnapshotCatalogPath: "./snapshots/index.json",
};

class EarthTexture {
  constructor(url) {
    this.url = url;
    this.textureCanvas = document.createElement("canvas");
    this.textureContext = this.textureCanvas.getContext("2d", { willReadFrequently: true });
    this.outputCanvas = document.createElement("canvas");
    this.outputContext = this.outputCanvas.getContext("2d", { willReadFrequently: true });
    this.outputFrame = null;
    this.textureData = null;
    this.loadPromise = null;
    this.ready = false;
    this.lastKey = "";
  }

  ensureLoaded() {
    if (this.loadPromise) {
      return this.loadPromise;
    }

    this.loadPromise = new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = () => {
        this.textureCanvas.width = image.naturalWidth;
        this.textureCanvas.height = image.naturalHeight;
        this.textureContext.drawImage(image, 0, 0);
        this.textureData = this.textureContext.getImageData(0, 0, image.naturalWidth, image.naturalHeight);
        this.ready = true;
        resolve();
      };
      image.onerror = () => reject(new Error(`Failed to load Earth texture: ${this.url}`));
      image.src = this.url;
    });

    return this.loadPromise;
  }

  // Render the equirectangular texture into the visible Earth hemisphere using
  // the same yaw and pitch as the satellite projection.
  render(yaw, pitch, globeRadius) {
    if (!this.ready || !this.textureData) {
      return null;
    }

    const targetSize = clamp(Math.round(globeRadius * 2), EARTH_TEXTURE_MIN_SIZE, EARTH_TEXTURE_MAX_SIZE);
    const cache = getEarthTextureCache(targetSize);
    const key = `${targetSize}:${yaw.toFixed(3)}:${pitch.toFixed(3)}`;
    if (this.lastKey === key) {
      return this.outputCanvas;
    }

    if (this.outputCanvas.width !== targetSize || this.outputCanvas.height !== targetSize) {
      this.outputCanvas.width = targetSize;
      this.outputCanvas.height = targetSize;
      this.outputFrame = this.outputContext.createImageData(targetSize, targetSize);
    }

    const output = this.outputFrame.data;
    output.fill(0);
    const texture = this.textureData.data;
    const textureWidth = this.textureData.width;
    const textureHeight = this.textureData.height;

    for (const sample of cache.samples) {
      const world = inverseRotateECEF(sample.view, yaw, pitch);
      const longitude = Math.atan2(world.y, world.x);
      const latitude = Math.asin(clamp(world.z, -1, 1));
      const textureOffset = sampleTextureOffset(textureWidth, textureHeight, longitude, latitude);

      output[sample.offset] = texture[textureOffset];
      output[sample.offset + 1] = texture[textureOffset + 1];
      output[sample.offset + 2] = texture[textureOffset + 2];
      output[sample.offset + 3] = texture[textureOffset + 3];
    }

    this.outputContext.putImageData(this.outputFrame, 0, 0);
    this.lastKey = key;
    return this.outputCanvas;
  }
}

class SnapshotModel {
  constructor(metadata) {
    this.metadata = metadata;
    this.states = metadata.States ?? [];
    this.links = metadata.Links ?? [];
    this.satelliteNames = new Set((metadata.Satellites ?? []).map((item) => item.Name));
    this.groundNames = new Set((metadata.Grounds ?? []).map((item) => item.Name));
    this.nodeInfo = new Map();

    for (const satellite of metadata.Satellites ?? []) {
      this.nodeInfo.set(satellite.Name, {
        name: satellite.Name,
        kind: "Satellite",
        computingType: satellite.ComputingType,
      });
    }
    for (const ground of metadata.Grounds ?? []) {
      this.nodeInfo.set(ground.Name, {
        name: ground.Name,
        kind: "Ground",
        computingType: ground.ComputingType,
      });
    }
  }

  static async load(url) {
    const response = await fetch(url, { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`Failed to load snapshot: ${response.status} ${response.statusText}`);
    }
    const metadata = await response.json();
    return new SnapshotModel(metadata);
  }

  get frameCount() {
    return this.states.length;
  }

  getFrame(index) {
    return this.states[index] ?? null;
  }

  getNodeInfo(name) {
    return this.nodeInfo.get(name) ?? { name, kind: "Unknown", computingType: "Unknown" };
  }

  getFrameLinks(frame) {
    const indices = new Set();
    for (const nodeState of frame.NodeStates ?? []) {
      for (const linkIndex of nodeState.Established ?? []) {
        indices.add(linkIndex);
      }
    }
    return [...indices].map((index) => ({ index, ...this.links[index] })).filter((link) => link.NodeName1 && link.NodeName2);
  }
}

class GlobeRenderer {
  constructor(canvas, onNodeSelected) {
    this.canvas = canvas;
    this.context = canvas.getContext("2d");
    this.onNodeSelected = onNodeSelected;
    this.earthTexture = null;
    this.yaw = -0.8;
    this.pitch = 0.35;
    this.zoom = 1;
    this.autoRotate = true;
    this.showSatellites = true;
    this.showGrounds = true;
    this.showLinks = true;
    this.showHidden = false;
    this.showLabels = false;
    this.maxSatellites = 0;
    this.hoverNodes = [];
    this.dragState = null;
    this.lastRenderAt = performance.now();

    this.resizeObserver = new ResizeObserver(() => this.resize());
    this.resizeObserver.observe(this.canvas);
    this.resize();
    this.bindEvents();
    this.setEarthTexture(DEFAULT_VIEWER_CONFIG.EarthImagePath);
  }

  setEarthTexture(url) {
    if (this.earthTexture?.url === url) {
      return;
    }
    this.earthTexture = new EarthTexture(url);
    this.earthTexture.ensureLoaded().catch((error) => console.error(error));
  }

  bindEvents() {
    this.canvas.addEventListener("pointerdown", (event) => {
      this.dragState = {
        x: event.clientX,
        y: event.clientY,
        yaw: this.yaw,
        pitch: this.pitch,
        moved: false,
      };
      this.canvas.classList.add("dragging");
      this.canvas.setPointerCapture(event.pointerId);
    });

    this.canvas.addEventListener("pointermove", (event) => {
      if (!this.dragState) {
        return;
      }
      const dx = event.clientX - this.dragState.x;
      const dy = event.clientY - this.dragState.y;
      this.yaw = this.dragState.yaw + dx * 0.005;
      this.pitch = clamp(this.dragState.pitch + dy * 0.005, -Math.PI / 2.1, Math.PI / 2.1);
      this.dragState.moved = true;
    });

    this.canvas.addEventListener("pointerup", (event) => {
      this.canvas.classList.remove("dragging");
      this.canvas.releasePointerCapture(event.pointerId);
      const dragState = this.dragState;
      this.dragState = null;
      if (!dragState?.moved) {
        this.pickNode(event.offsetX, event.offsetY);
      }
    });

    this.canvas.addEventListener("pointerleave", () => {
      this.canvas.classList.remove("dragging");
      this.dragState = null;
    });

    this.canvas.addEventListener(
      "wheel",
      (event) => {
        event.preventDefault();
        const direction = event.deltaY > 0 ? -1 : 1;
        this.zoom = clamp(this.zoom + direction * 0.08, 0.65, 1.9);
      },
      { passive: false },
    );
  }

  resize() {
    const devicePixelRatio = window.devicePixelRatio || 1;
    const bounds = this.canvas.getBoundingClientRect();
    this.canvas.width = Math.max(1, Math.round(bounds.width * devicePixelRatio));
    this.canvas.height = Math.max(1, Math.round(bounds.height * devicePixelRatio));
    this.context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
    this.width = bounds.width;
    this.height = bounds.height;
  }

  setOptions(options) {
    this.autoRotate = options.autoRotate;
    this.showSatellites = options.showSatellites;
    this.showGrounds = options.showGrounds;
    this.showLinks = options.showLinks;
    this.showHidden = options.showHidden;
    this.showLabels = options.showLabels;
    this.maxSatellites = options.maxSatellites;
  }

  resetView() {
    this.yaw = -0.8;
    this.pitch = 0.35;
    this.zoom = 1;
  }

  render(model, frame, selectedName) {
    if (!frame) {
      return;
    }

    const now = performance.now();
    const dtSeconds = (now - this.lastRenderAt) / 1000;
    this.lastRenderAt = now;
    if (this.autoRotate && !this.dragState) {
      this.yaw += dtSeconds * 0.08;
    }

    const ctx = this.context;
    ctx.clearRect(0, 0, this.width, this.height);

    const centerX = this.width / 2;
    const centerY = this.height / 2;
    const globeRadius = Math.min(this.width, this.height) * 0.36 * this.zoom;
    const meterScale = globeRadius / EARTH_RADIUS_METERS;

    this.drawBackdrop(ctx, centerX, centerY, globeRadius);
    this.drawEarth(ctx, centerX, centerY, globeRadius);
    this.drawGraticule(ctx, centerX, centerY, meterScale);

    const projectedNodes = this.projectNodes(model, frame, centerX, centerY, meterScale);
    const scene = buildRenderableScene(projectedNodes, {
      showSatellites: this.showSatellites,
      showGrounds: this.showGrounds,
      maxSatellites: this.maxSatellites,
      selectedName,
    });
    const links = this.showLinks ? model.getFrameLinks(frame) : [];
    let linkCount = 0;
    if (links.length > 0) {
      linkCount = this.drawLinks(ctx, scene.nodes, links, selectedName);
    }
    this.drawNodes(ctx, scene.nodes, selectedName);

    if (this.showLabels) {
      this.drawLabels(ctx, scene.nodes);
    }

    this.hoverNodes = scene.nodes;
    return {
      ...scene,
      linkCount,
    };
  }

  drawBackdrop(ctx, centerX, centerY, globeRadius) {
    const halo = ctx.createRadialGradient(centerX, centerY, globeRadius * 0.4, centerX, centerY, globeRadius * 1.5);
    halo.addColorStop(0, "rgba(255,255,255,0.08)");
    halo.addColorStop(1, "rgba(255,255,255,0)");
    ctx.fillStyle = halo;
    ctx.beginPath();
    ctx.arc(centerX, centerY, globeRadius * 1.55, 0, Math.PI * 2);
    ctx.fill();
  }

  drawEarth(ctx, centerX, centerY, globeRadius) {
    const gradient = ctx.createRadialGradient(
      centerX - globeRadius * 0.32,
      centerY - globeRadius * 0.34,
      globeRadius * 0.08,
      centerX,
      centerY,
      globeRadius,
    );
    gradient.addColorStop(0, "rgba(166, 214, 223, 0.98)");
    gradient.addColorStop(0.5, "rgba(36, 96, 119, 0.38)");
    gradient.addColorStop(1, "rgba(6, 22, 32, 0.72)");

    ctx.save();
    ctx.beginPath();
    ctx.arc(centerX, centerY, globeRadius, 0, Math.PI * 2);
    ctx.clip();

    ctx.fillStyle = "rgba(25, 102, 123, 1)";
    ctx.fillRect(centerX - globeRadius, centerY - globeRadius, globeRadius * 2, globeRadius * 2);

    const texturedEarth = this.earthTexture.render(this.yaw, this.pitch, globeRadius);
    if (texturedEarth) {
      ctx.drawImage(texturedEarth, centerX - globeRadius, centerY - globeRadius, globeRadius * 2, globeRadius * 2);
    }

    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(centerX, centerY, globeRadius, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    ctx.strokeStyle = "rgba(255,255,255,0.16)";
    ctx.lineWidth = 1.2;
    ctx.beginPath();
    ctx.arc(centerX, centerY, globeRadius, 0, Math.PI * 2);
    ctx.stroke();
  }

  drawGraticule(ctx, centerX, centerY, meterScale) {
    ctx.save();
    ctx.strokeStyle = "rgba(255,255,255,0.18)";
    ctx.lineWidth = 1;

    for (let latitude = -60; latitude <= 60; latitude += 30) {
      this.drawLatitudeLine(ctx, centerX, centerY, meterScale, latitude);
    }
    for (let longitude = 0; longitude < 180; longitude += 30) {
      this.drawLongitudeLine(ctx, centerX, centerY, meterScale, longitude);
    }
    ctx.restore();
  }

  drawLatitudeLine(ctx, centerX, centerY, meterScale, latitudeDeg) {
    const points = [];
    for (let longitude = -180; longitude <= 180; longitude += 4) {
      const world = latLonToECEF(latitudeDeg, longitude, EARTH_RADIUS_METERS);
      points.push(this.projectPoint(world, centerX, centerY, meterScale));
    }
    drawProjectedPolyline(ctx, points, this.showHidden);
  }

  drawLongitudeLine(ctx, centerX, centerY, meterScale, longitudeDeg) {
    const points = [];
    for (let latitude = -90; latitude <= 90; latitude += 4) {
      const world = latLonToECEF(latitude, longitudeDeg, EARTH_RADIUS_METERS);
      points.push(this.projectPoint(world, centerX, centerY, meterScale));
    }
    drawProjectedPolyline(ctx, points, this.showHidden);
  }

  projectNodes(model, frame, centerX, centerY, meterScale) {
    const nodes = [];
    for (const nodeState of frame.NodeStates ?? []) {
      const info = model.getNodeInfo(nodeState.Name);
      const projected = this.projectPoint(nodeState.Position, centerX, centerY, meterScale);
      const altitudeMeters = vectorLength(nodeState.Position) - EARTH_RADIUS_METERS;
      nodes.push({
        ...projected,
        name: nodeState.Name,
        kind: info.kind,
        computingType: info.computingType,
        position: nodeState.Position,
        altitudeMeters,
        established: nodeState.Established ?? [],
      });
    }

    nodes.sort((a, b) => a.depth - b.depth);
    return nodes;
  }

  projectPoint(position, centerX, centerY, meterScale) {
    const rotated = rotateECEF(position, this.yaw, this.pitch);
    return {
      x: centerX + rotated.y * meterScale,
      y: centerY - rotated.z * meterScale,
      depth: rotated.x,
      visible: rotated.x >= 0,
    };
  }

  drawLinks(ctx, projectedNodes, links, selectedName) {
    const nodeMap = new Map(projectedNodes.map((node) => [node.name, node]));
    let drawnCount = 0;

    ctx.save();
    for (const link of links) {
      const nodeA = nodeMap.get(link.NodeName1);
      const nodeB = nodeMap.get(link.NodeName2);
      if (!nodeA || !nodeB) {
        continue;
      }
      if (!this.showHidden && (!nodeA.visible || !nodeB.visible)) {
        continue;
      }

      const highlighted = selectedName && (nodeA.name === selectedName || nodeB.name === selectedName);
      ctx.strokeStyle = highlighted ? "rgba(255, 224, 102, 0.95)" : "rgba(245, 176, 65, 0.42)";
      ctx.lineWidth = highlighted ? 2.2 : 1.2;
      ctx.beginPath();
      ctx.moveTo(nodeA.x, nodeA.y);
      ctx.lineTo(nodeB.x, nodeB.y);
      ctx.stroke();
      drawnCount += 1;
    }
    ctx.restore();
    return drawnCount;
  }

  drawNodes(ctx, nodes, selectedName) {
    for (const node of nodes) {
      if (!this.showHidden && !node.visible) {
        continue;
      }

      const isSelected = node.name === selectedName;
      const color = node.kind === "Ground" ? "#2a9d8f" : "#ff7a59";
      const radius = node.kind === "Ground" ? 4.4 : 3.2;
      const alpha = node.visible ? 1 : 0.28;

      ctx.save();
      ctx.fillStyle = applyAlpha(color, alpha);
      ctx.beginPath();
      ctx.arc(node.x, node.y, isSelected ? radius + 2.2 : radius, 0, Math.PI * 2);
      ctx.fill();
      if (isSelected) {
        ctx.strokeStyle = "rgba(255, 240, 214, 0.95)";
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(node.x, node.y, radius + 5.5, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.restore();
    }
  }

  drawLabels(ctx, nodes) {
    ctx.save();
    ctx.fillStyle = "rgba(255, 249, 238, 0.92)";
    ctx.font = '12px "Avenir Next", "Segoe UI", sans-serif';
    for (const node of nodes) {
      if (!this.showHidden && !node.visible) {
        continue;
      }
      ctx.fillText(node.name, node.x + 7, node.y - 7);
    }
    ctx.restore();
  }

  pickNode(pointerX, pointerY) {
    let selected = null;
    let bestDistance = 14;
    for (const node of this.hoverNodes) {
      if (!this.showHidden && !node.visible) {
        continue;
      }
      const distance = Math.hypot(node.x - pointerX, node.y - pointerY);
      if (distance < bestDistance) {
        bestDistance = distance;
        selected = node;
      }
    }
    this.onNodeSelected(selected?.name ?? null);
  }
}

class ViewerApp {
  constructor() {
    this.catalog = [];
    this.config = { ...DEFAULT_VIEWER_CONFIG };
    this.model = null;
    this.frameIndex = 0;
    this.playing = false;
    this.selectedName = null;
    this.selectedSnapshotID = "";
    this.lastScene = null;
    this.playbackAccumulator = 0;
    this.lastFrameTime = performance.now();

    this.elements = {
      canvas: document.getElementById("globe-canvas"),
      playButton: document.getElementById("play-button"),
      resetViewButton: document.getElementById("reset-view-button"),
      frameSlider: document.getElementById("frame-slider"),
      frameLabel: document.getElementById("frame-label"),
      frameCount: document.getElementById("frame-count"),
      satelliteCount: document.getElementById("satellite-count"),
      groundCount: document.getElementById("ground-count"),
      linkCount: document.getElementById("link-count"),
      snapshotSelect: document.getElementById("snapshot-select"),
      snapshotMeta: document.getElementById("snapshot-meta"),
      timeReadout: document.getElementById("time-readout"),
      statusPill: document.getElementById("status-pill"),
      playbackSpeed: document.getElementById("playback-speed"),
      maxSatellites: document.getElementById("max-satellites"),
      maxSatellitesHint: document.getElementById("max-satellites-hint"),
      showSatellites: document.getElementById("show-satellites"),
      showGrounds: document.getElementById("show-grounds"),
      showLinks: document.getElementById("show-links"),
      showHidden: document.getElementById("show-hidden"),
      showLabels: document.getElementById("show-labels"),
      autoRotate: document.getElementById("auto-rotate"),
      selectionEmpty: document.getElementById("selection-empty"),
      selectionDetails: document.getElementById("selection-details"),
      selectionName: document.getElementById("selection-name"),
      selectionType: document.getElementById("selection-type"),
      selectionAltitude: document.getElementById("selection-altitude"),
      selectionLinks: document.getElementById("selection-links"),
      selectionPosition: document.getElementById("selection-position"),
    };

    this.renderer = new GlobeRenderer(this.elements.canvas, (name) => {
      this.selectedName = name;
      this.renderSelection();
    });

    this.bindControls();
  }

  async loadConfig() {
    const response = await fetch(VIEWER_CONFIG_PATH, { cache: "no-store" });
    if (!response.ok) {
      return { ...DEFAULT_VIEWER_CONFIG };
    }
    return { ...DEFAULT_VIEWER_CONFIG, ...(await response.json()) };
  }

  async loadCatalog() {
    const response = await fetch(this.config.SnapshotCatalogPath, { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`Failed to load snapshot catalog: ${response.status} ${response.statusText}`);
    }
    return response.json();
  }

  async loadSnapshot(snapshotID) {
    this.elements.statusPill.textContent = "Loading snapshot…";

    const entry = this.lookupSnapshotEntry(snapshotID);
    if (!entry) {
      throw new Error(`Unknown snapshot: ${snapshotID}`);
    }

    const model = await SnapshotModel.load(entry.Path);
    this.model = model;
    this.selectedSnapshotID = snapshotID;
    this.frameIndex = 0;
    this.selectedName = null;
    this.playing = false;
    this.playbackAccumulator = 0;
    this.elements.playButton.textContent = "Play";
    this.elements.frameSlider.value = "0";

    if (this.model.frameCount === 0) {
      this.elements.frameSlider.max = "0";
      this.elements.frameCount.textContent = "0";
      this.elements.satelliteCount.textContent = String(this.model.satelliteNames.size);
      this.elements.groundCount.textContent = String(this.model.groundNames.size);
      this.normalizeSatelliteLimit();
      this.renderSnapshotCatalog();
      return;
    }

    this.elements.frameSlider.max = String(this.model.frameCount - 1);
    this.elements.frameCount.textContent = String(this.model.frameCount);
    this.elements.satelliteCount.textContent = String(this.model.satelliteNames.size);
    this.elements.groundCount.textContent = String(this.model.groundNames.size);
    this.normalizeSatelliteLimit();
    this.renderSnapshotCatalog();
  }

  renderSnapshotCatalog() {
    const select = this.elements.snapshotSelect;
    const meta = this.elements.snapshotMeta;
    select.replaceChildren();

    if (this.catalog.length === 0) {
      select.disabled = true;
      meta.textContent = "No precomputed snapshots available.";
      return;
    }

    select.disabled = false;
    for (const entry of this.catalog) {
      const option = document.createElement("option");
      option.value = entry.ID;
      option.textContent = formatSnapshotLabel(entry);
      select.append(option);
    }

    select.value = this.selectedSnapshotID;
    const selectedEntry = this.catalog.find((entry) => entry.ID === this.selectedSnapshotID) ?? this.catalog[0];
    if (selectedEntry && select.value !== selectedEntry.ID) {
      select.value = selectedEntry.ID;
    }
    if (selectedEntry) {
      meta.textContent = `${selectedEntry.GroundCount} ground stations · ${selectedEntry.FrameCount} frames · ${selectedEntry.Filename}`;
    }
  }

  lookupSnapshotEntry(snapshotID) {
    return this.catalog.find((entry) => entry.ID === snapshotID) ?? null;
  }

  bindControls() {
    this.elements.playButton.addEventListener("click", () => {
      this.playing = !this.playing;
      this.elements.playButton.textContent = this.playing ? "Pause" : "Play";
    });

    this.elements.resetViewButton.addEventListener("click", () => {
      this.renderer.resetView();
    });

    this.elements.frameSlider.addEventListener("input", (event) => {
      this.frameIndex = Number(event.target.value);
      this.playing = false;
      this.elements.playButton.textContent = "Play";
    });

    this.elements.maxSatellites.addEventListener("input", () => {
      this.normalizeSatelliteLimit();
    });

    this.elements.snapshotSelect.addEventListener("change", (event) => {
      const snapshotID = event.target.value;
      if (!snapshotID || snapshotID === this.selectedSnapshotID) {
        return;
      }

      this.loadSnapshot(snapshotID).catch((error) => {
        console.error(error);
        this.elements.statusPill.textContent = "Failed to load snapshot";
        this.elements.timeReadout.textContent = error.message;
        this.renderSnapshotCatalog();
      });
    });
  }

  normalizeSatelliteLimit() {
    const totalSatellites = this.model?.satelliteNames.size ?? 0;
    const normalizedLimit = parseSatelliteLimit(this.elements.maxSatellites.value, totalSatellites);
    this.elements.maxSatellites.value = String(normalizedLimit);
    this.elements.maxSatellitesHint.textContent =
      normalizedLimit === 0
        ? `0 = all satellites (${totalSatellites} total)`
        : `${normalizedLimit} of ${totalSatellites} satellites`;
  }

  readMaxSatellites() {
    return parseSatelliteLimit(this.elements.maxSatellites.value, this.model?.satelliteNames.size ?? 0);
  }

  syncSelectionWithScene(scene) {
    if (!this.selectedName || !scene) {
      return;
    }
    if (!scene.nodeNames.has(this.selectedName)) {
      this.selectedName = null;
    }
  }

  async start() {
    try {
      this.config = await this.loadConfig();
      this.renderer.setEarthTexture(this.config.EarthImagePath);
      const catalogResponse = await this.loadCatalog();
      this.catalog = catalogResponse.Snapshots ?? [];
      this.selectedSnapshotID = catalogResponse.SelectedID ?? this.catalog[0]?.ID ?? "";
      this.renderSnapshotCatalog();
      if (!this.selectedSnapshotID) {
        this.elements.statusPill.textContent = "No snapshots found";
        this.renderStatic();
        return;
      }
      await this.loadSnapshot(this.selectedSnapshotID);
    } catch (error) {
      this.elements.statusPill.textContent = "Failed to load snapshot";
      this.elements.timeReadout.textContent = error.message;
      throw error;
    }

    if (!this.model || this.model.frameCount === 0) {
      this.elements.statusPill.textContent = "Snapshot is empty";
      this.renderStatic();
      return;
    }
    this.animate();
  }

  animate() {
    requestAnimationFrame(() => this.animate());

    if (!this.model || this.model.frameCount === 0) {
      this.renderStatic();
      return;
    }

    const now = performance.now();
    const deltaMs = now - this.lastFrameTime;
    this.lastFrameTime = now;

    if (this.playing) {
      const speed = Number(this.elements.playbackSpeed.value);
      this.playbackAccumulator += deltaMs * speed;
      if (this.playbackAccumulator >= 750) {
        this.playbackAccumulator = 0;
        this.frameIndex = (this.frameIndex + 1) % this.model.frameCount;
        this.elements.frameSlider.value = String(this.frameIndex);
      }
    }

    this.renderer.setOptions({
      autoRotate: this.elements.autoRotate.checked,
      showSatellites: this.elements.showSatellites.checked,
      showGrounds: this.elements.showGrounds.checked,
      showLinks: this.elements.showLinks.checked,
      showHidden: this.elements.showHidden.checked,
      showLabels: this.elements.showLabels.checked,
      maxSatellites: this.readMaxSatellites(),
    });
    this.lastScene = this.renderer.render(this.model, this.model.getFrame(this.frameIndex), this.selectedName);
    this.syncSelectionWithScene(this.lastScene);
    this.renderFrameMeta(this.lastScene);
    this.renderSelection();
  }

  renderStatic() {
    this.renderer.setOptions({
      autoRotate: this.elements.autoRotate.checked,
      showSatellites: false,
      showGrounds: false,
      showLinks: false,
      showHidden: false,
      showLabels: false,
      maxSatellites: 0,
    });
    this.lastScene = this.renderer.render(
      new SnapshotModel({ States: [{ NodeStates: [] }], Links: [], Satellites: [], Grounds: [] }),
      { NodeStates: [] },
      null,
    );
  }

  renderFrameMeta(scene) {
    if (!this.model || this.model.frameCount === 0) {
      return;
    }
    const frame = this.model.getFrame(this.frameIndex);
    this.elements.frameLabel.textContent = `Frame ${this.frameIndex + 1} / ${this.model.frameCount}`;
    this.elements.linkCount.textContent = String(scene?.linkCount ?? 0);
    this.elements.timeReadout.textContent = `Time: ${formatTimestamp(frame.Time)}`;
    this.elements.statusPill.textContent = formatSceneSummary(
      scene,
      this.model.satelliteNames.size,
      this.model.groundNames.size,
    );
  }

  renderSelection() {
    if (!this.model || this.model.frameCount === 0 || !this.selectedName) {
      this.elements.selectionEmpty.classList.remove("hidden");
      this.elements.selectionDetails.classList.add("hidden");
      return;
    }

    const frame = this.model.getFrame(this.frameIndex);
    const nodeState = (frame.NodeStates ?? []).find((node) => node.Name === this.selectedName);
    if (!nodeState) {
      this.elements.selectionEmpty.classList.remove("hidden");
      this.elements.selectionDetails.classList.add("hidden");
      return;
    }

    const info = this.model.getNodeInfo(nodeState.Name);
    const altitudeKm = (vectorLength(nodeState.Position) - EARTH_RADIUS_METERS) / 1000;

    this.elements.selectionEmpty.classList.add("hidden");
    this.elements.selectionDetails.classList.remove("hidden");
    this.elements.selectionName.textContent = info.name;
    this.elements.selectionType.textContent = `${info.kind} · ${info.computingType ?? "Unknown"}`;
    this.elements.selectionAltitude.textContent = `${altitudeKm.toFixed(1)} km`;
    this.elements.selectionLinks.textContent = String((nodeState.Established ?? []).length);
    this.elements.selectionPosition.textContent = formatVector(nodeState.Position);
  }
}

// Keep the filtering decision outside the renderer drawing methods so links,
// picking, and labels all operate on the same node subset.
function buildRenderableScene(nodes, options) {
  const satellites = [];
  const grounds = [];
  const others = [];

  for (const node of nodes) {
    if (node.kind === "Satellite") {
      if (options.showSatellites) {
        satellites.push(node);
      }
      continue;
    }
    if (node.kind === "Ground") {
      if (options.showGrounds) {
        grounds.push(node);
      }
      continue;
    }
    others.push(node);
  }

  const limitedSatellites = limitSatellites(satellites, options.maxSatellites, options.selectedName);
  const renderNodes = [...grounds, ...limitedSatellites, ...others].sort((left, right) => left.depth - right.depth);

  return {
    nodes: renderNodes,
    nodeNames: new Set(renderNodes.map((node) => node.name)),
    satelliteCount: limitedSatellites.length,
    groundCount: grounds.length,
  };
}

function limitSatellites(satellites, maxSatellites, selectedName) {
  if (maxSatellites === 0 || satellites.length <= maxSatellites) {
    return satellites;
  }

  const prioritized = [...satellites].sort(compareSatellitePriority);
  const limited = prioritized.slice(0, maxSatellites);

  if (!selectedName || limited.some((node) => node.name === selectedName)) {
    return limited;
  }

  const selectedNode = satellites.find((node) => node.name === selectedName);
  if (!selectedNode) {
    return limited;
  }

  limited[limited.length - 1] = selectedNode;
  return limited;
}

function compareSatellitePriority(left, right) {
  if (left.visible !== right.visible) {
    return left.visible ? -1 : 1;
  }
  if (left.depth !== right.depth) {
    return right.depth - left.depth;
  }
  return left.name.localeCompare(right.name);
}

function parseSatelliteLimit(rawValue, totalSatellites) {
  const parsed = Number.parseInt(rawValue, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return clamp(parsed, 0, totalSatellites);
}

function formatSceneSummary(scene, totalSatellites, totalGrounds) {
  if (!scene) {
    return "Snapshot loaded";
  }
  return `Showing ${scene.satelliteCount}/${totalSatellites} sat · ${scene.groundCount}/${totalGrounds} ground`;
}

function formatSnapshotLabel(entry) {
  return `${entry.SatelliteCount} satellites`;
}

function rotateECEF(position, yaw, pitch) {
  const cosYaw = Math.cos(yaw);
  const sinYaw = Math.sin(yaw);
  const xYaw = cosYaw * position.X - sinYaw * position.Y;
  const yYaw = sinYaw * position.X + cosYaw * position.Y;
  const zYaw = position.Z;

  const cosPitch = Math.cos(pitch);
  const sinPitch = Math.sin(pitch);
  return {
    x: cosPitch * xYaw + sinPitch * zYaw,
    y: yYaw,
    z: -sinPitch * xYaw + cosPitch * zYaw,
  };
}

function inverseRotateECEF(position, yaw, pitch) {
  const cosPitch = Math.cos(pitch);
  const sinPitch = Math.sin(pitch);
  const xYaw = cosPitch * position.x - sinPitch * position.z;
  const zYaw = sinPitch * position.x + cosPitch * position.z;

  const cosYaw = Math.cos(yaw);
  const sinYaw = Math.sin(yaw);
  return {
    x: cosYaw * xYaw + sinYaw * position.y,
    y: -sinYaw * xYaw + cosYaw * position.y,
    z: zYaw,
  };
}

function latLonToECEF(latitudeDeg, longitudeDeg, radius) {
  const latitude = latitudeDeg * DEG_TO_RAD;
  const longitude = longitudeDeg * DEG_TO_RAD;
  return {
    X: radius * Math.cos(latitude) * Math.cos(longitude),
    Y: radius * Math.cos(latitude) * Math.sin(longitude),
    Z: radius * Math.sin(latitude),
  };
}

function drawProjectedPolyline(ctx, points, showHidden) {
  let drawing = false;
  ctx.beginPath();

  for (const point of points) {
    if (!showHidden && !point.visible) {
      drawing = false;
      continue;
    }
    if (!drawing) {
      ctx.moveTo(point.x, point.y);
      drawing = true;
    } else {
      ctx.lineTo(point.x, point.y);
    }
  }

  ctx.stroke();
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function getEarthTextureCache(size) {
  if (EARTH_TEXTURE_CACHE.has(size)) {
    return EARTH_TEXTURE_CACHE.get(size);
  }

  const radius = size / 2;
  const samples = [];
  for (let y = 0; y < size; y += 1) {
    for (let x = 0; x < size; x += 1) {
      const normalizedY = (x + 0.5 - radius) / radius;
      const normalizedZ = -(y + 0.5 - radius) / radius;
      const distanceSquared = normalizedY * normalizedY + normalizedZ * normalizedZ;
      if (distanceSquared > 1) {
        continue;
      }

      samples.push({
        offset: (y * size + x) * 4,
        view: {
          x: Math.sqrt(1 - distanceSquared),
          y: normalizedY,
          z: normalizedZ,
        },
      });
    }
  }

  const cache = { samples };
  EARTH_TEXTURE_CACHE.set(size, cache);
  return cache;
}

function sampleTextureOffset(textureWidth, textureHeight, longitude, latitude) {
  const normalizedLongitude = (longitude + Math.PI) / (Math.PI * 2);
  const normalizedLatitude = (Math.PI / 2 - latitude) / Math.PI;

  const x = Math.floor(((normalizedLongitude % 1 + 1) % 1) * (textureWidth - 1));
  const y = Math.floor(clamp(normalizedLatitude, 0, 1) * (textureHeight - 1));
  return (y * textureWidth + x) * 4;
}

function vectorLength(position) {
  return Math.sqrt(position.X * position.X + position.Y * position.Y + position.Z * position.Z);
}

function formatVector(position) {
  return `${(position.X / 1000).toFixed(0)} km, ${(position.Y / 1000).toFixed(0)} km, ${(position.Z / 1000).toFixed(0)} km`;
}

function formatTimestamp(value) {
  const date = new Date(value);
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "medium",
    timeZone: "UTC",
  }).format(date) + " UTC";
}

function applyAlpha(hexColor, alpha) {
  const normalized = hexColor.replace("#", "");
  const red = parseInt(normalized.slice(0, 2), 16);
  const green = parseInt(normalized.slice(2, 4), 16);
  const blue = parseInt(normalized.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

const app = new ViewerApp();
app.start().catch((error) => {
  console.error(error);
});
