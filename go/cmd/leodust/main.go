package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/internal/deployment"
	"github.com/leotrek/leodust/internal/ground"
	"github.com/leotrek/leodust/internal/orbit"
	"github.com/leotrek/leodust/internal/routing"
	"github.com/leotrek/leodust/internal/satellite"
	"github.com/leotrek/leodust/internal/simplugin"
	"github.com/leotrek/leodust/internal/simulation"
	"github.com/leotrek/leodust/internal/stateplugin"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

func main() {
	simulationConfigString := flag.String(
		"simulationConfig",
		"./resources/configs/simulationAutorunConfig.yaml",
		"Path to the simulation config file",
	)
	islConfigString := flag.String(
		"islConfig",
		"./resources/configs/islMstConfig.yaml",
		"Path to inter satellite link config file",
	)
	groundLinkConfigString := flag.String(
		"groundLinkConfig",
		"./resources/configs/groundLinkNearestConfig.yaml",
		"Path to ground link config file",
	)
	computingConfigString := flag.String(
		"computingConfig",
		"./resources/configs/computingConfig.yaml",
		"Path to computing config file",
	)
	routerConfigString := flag.String(
		"routerConfig",
		"./resources/configs/routerAStarConfig.yaml",
		"Path to router config file",
	)
	simulationStateOutputFile := flag.String(
		"simulationStateOutputFile",
		"./simulation_state_output.gob",
		"Path to output the simulation state (optional)",
	)
	simulationStateInputFile := flag.String(
		"simulationStateInputFile",
		"",
		"Path to input the simulation state (optional)",
	)
	simulationPluginString := flag.String(
		"simulationPlugins",
		"",
		"Plugin names (optional, comma-separated list)",
	)
	statePluginString := flag.String(
		"statePlugins",
		"",
		"Plugin names (optional, comma-separated list)",
	)
	topologyOutputFile := flag.String(
		"topologyOutputFile",
		"",
		"Path to continuously export live topology snapshots when TopologyExportPlugin is enabled",
	)
	runtimeOutputFile := flag.String(
		"runtimeOutputFile",
		"",
		"Path to continuously export runtime reconciliation snapshots when RuntimeReconcilePlugin is enabled",
	)
	logLevelFlag := flag.String(
		"logLevel",
		"",
		"Log level override: error, warn, info, debug",
	)
	validateOrbitFlag := flag.Bool(
		"validateOrbit",
		false,
		"Run the built-in orbit validation suite before starting the simulation",
	)
	validateOrbitOnlyFlag := flag.Bool(
		"validateOrbitOnly",
		false,
		"Run the built-in orbit validation suite and exit",
	)
	downloadTLETodayFlag := flag.Bool(
		"downloadTLEToday",
		false,
		"Download a fresh TLE snapshot before starting the simulation",
	)
	downloadTLEGroupFlag := flag.String(
		"downloadTLEGroup",
		"starlink",
		"CelesTrak TLE group to download when --downloadTLEToday is set",
	)
	downloadTLEOutputFlag := flag.String(
		"downloadTLEOutput",
		"",
		"Optional output path for the downloaded TLE file",
	)
	flag.Parse()

	simulationPluginList := parseListFlag(*simulationPluginString)
	statePluginList := parseListFlag(*statePluginString)

	// Step 1: Load configuration
	simulationConfig := mustLoadConfig[configs.SimulationConfig](*simulationConfigString, "simulation configuration")
	computingConfig := mustLoadConfig[[]configs.ComputingConfig](*computingConfigString, "computing configuration")
	routerConfig := mustLoadConfig[configs.RouterConfig](*routerConfigString, "router configuration")
	configureLogging(simulationConfig.LogLevel, *logLevelFlag)

	tleSummaryLogged := false
	if *downloadTLETodayFlag {
		outputPath := *downloadTLEOutputFlag
		downloadedAt := time.Now().UTC()
		if outputPath == "" {
			outputPath = defaultDownloadedTLEPath(*downloadTLEGroupFlag, downloadedAt)
		}

		result, err := satellite.DownloadCurrentTLE(nil, *downloadTLEGroupFlag, outputPath, downloadedAt)
		if err != nil {
			logging.Fatalf("Failed to download current TLE snapshot: %v", err)
		}

		logging.Infof(
			"Downloaded current %s TLE snapshot to %s at %s",
			result.Group,
			result.Path,
			result.DownloadedAt.Format(time.RFC3339),
		)
		satellite.LogTLESummary(result.Summary, result.Path)
		satellite.WarnIfSimulationTimeFarFromTLE(result.Summary, simulationConfig.SimulationStartTime, result.Path)

		simulationConfig.SatelliteDataSourceType = "tle"
		simulationConfig.SatelliteDataSource = result.Path
		tleSummaryLogged = true
	}

	satelliteDataSourcePath := resolveDataSourcePath(simulationConfig.SatelliteDataSourceType, simulationConfig.SatelliteDataSource)

	if simulationConfig.SatelliteDataSourceType == "tle" && !tleSummaryLogged {
		logLocalTLESummary(satelliteDataSourcePath, simulationConfig.SimulationStartTime)
	}

	if *validateOrbitFlag || *validateOrbitOnlyFlag {
		if err := runOrbitValidation(); err != nil {
			logging.Fatalf("Orbit validation failed: %v", err)
		}
		if *validateOrbitOnlyFlag {
			return
		}
	}

	var simService types.SimulationController
	if *simulationStateInputFile != "" {
		simService = startSimulationIteration(*simulationConfig, *computingConfig, *routerConfig, *simulationStateInputFile, simulationPluginList, *topologyOutputFile, *runtimeOutputFile)
	} else {
		simService = startSimulation(*simulationConfig, *islConfigString, *groundLinkConfigString, *computingConfig, *routerConfig, simulationStateOutputFile, simulationPluginList, statePluginList, *topologyOutputFile, *runtimeOutputFile)
	}

	runController(simService, *simulationConfig)
}

func startSimulationIteration(simulationConfig configs.SimulationConfig, computingConfig []configs.ComputingConfig, routerConfig configs.RouterConfig, simulationStateInputFile string, simulationPluginList []string, topologyOutputFile string, runtimeOutputFile string) types.SimulationController {
	// Step 2: Build computing builder with configured strategies
	var computingBuilder computing.ComputingBuilder = computing.NewComputingBuilder(computingConfig)

	// Step 3: Build router builder
	routerBuilder := routing.NewRouterBuilder(routerConfig)

	// Step 4.1: Initialize plugin builder
	simPlugins := mustBuildSimulationPlugins(simulationPluginList, topologyOutputFile, runtimeOutputFile)

	// Step 5: State Plugin Builder
	statePluginBuilder := stateplugin.NewStatePluginPrecompBuilder(simulationStateInputFile)

	// Step 6: Inject orchestrator (if used)
	orchestrator := deployment.NewDeploymentOrchestrator()

	simStateDeserializer := simulation.NewSimulationStateDeserializer(&simulationConfig, simulationStateInputFile, computingBuilder, routerBuilder, orchestrator, simPlugins, statePluginBuilder)
	return simStateDeserializer.LoadIterator()
}

func startSimulation(simulationConfig configs.SimulationConfig, islConfigString string, groundLinkConfigString string, computingConfig []configs.ComputingConfig, routerConfig configs.RouterConfig, simulationStateOutputFile *string, simulationPluginList []string, statePluginList []string, topologyOutputFile string, runtimeOutputFile string) types.SimulationController {
	islConfig := mustLoadConfig[configs.InterSatelliteLinkConfig](islConfigString, "ISL configuration")
	groundLinkConfig := mustLoadConfig[configs.GroundLinkConfig](groundLinkConfigString, "ground-link configuration")
	satelliteDataSourcePath := resolveDataSourcePath(simulationConfig.SatelliteDataSourceType, simulationConfig.SatelliteDataSource)
	groundDataSourcePath := resolveDataSourcePath(simulationConfig.GroundStationDataSourceType, simulationConfig.GroundStationDataSource)

	// Step 2: Build computing builder with configured strategies
	computingBuilder := computing.NewComputingBuilder(computingConfig)

	// Step 3: Build router builder
	routerBuilder := routing.NewRouterBuilder(routerConfig)

	// Step 4.1: Initialize plugin builder
	simPlugins := mustBuildSimulationPlugins(simulationPluginList, topologyOutputFile, runtimeOutputFile)

	// Step 4.2: Initialize state plugin builder
	statePlugins := mustBuildStatePlugins(statePluginList)

	satBuilder := satellite.NewSatelliteBuilder(simulationConfig.SimulationStartTime, routerBuilder, computingBuilder, *islConfig)
	groundStationBuilder := ground.NewGroundStationBuilder(routerBuilder, computingBuilder, *groundLinkConfig)
	tleLoader := satellite.NewTleLoader(satBuilder)
	ymlLoader := ground.NewGroundStationYmlLoader(groundStationBuilder)

	constellationLoader := satellite.NewSatelliteConstellationLoader()
	constellationLoader.RegisterDataSourceLoader("tle", tleLoader)

	simService := simulation.NewSimulationService(&simulationConfig, simPlugins, types.NewStatePluginRepository(statePlugins), simulationStateOutputFile)

	orchestrator := deployment.NewDeploymentOrchestrator()
	simService.Inject(orchestrator)

	loaderService := satellite.NewSatelliteLoaderService(
		constellationLoader,
		simService,
		satelliteDataSourcePath,
		simulationConfig.SatelliteDataSourceType,
	)
	if err := loaderService.Start(); err != nil {
		logging.Fatalf("Failed to load satellites: %v", err)
	}

	groundLoaderService := ground.NewGroundStationLoaderService(
		simService,
		ymlLoader,
		groundDataSourcePath,
	)
	if err := groundLoaderService.Start(); err != nil {
		logging.Fatalf("Failed to load ground stations: %v", err)
	}

	return simService
}

// mustLoadConfig keeps the entrypoint linear by handling fatal startup errors at the edge.
func mustLoadConfig[T any](path, label string) *T {
	config, err := configs.LoadConfigFromFile[T](path)
	if err != nil {
		logging.Fatalf("Failed to load %s: %v", label, err)
	}
	return config
}

func configureLogging(configLevel, flagLevel string) {
	level := configLevel
	if flagLevel != "" {
		level = flagLevel
	}
	if err := logging.Configure(level); err != nil {
		logging.Fatalf("Failed to configure log level: %v", err)
	}
	logging.Infof("Log level set to %s", logging.CurrentLevel().String())
}

func runOrbitValidation() error {
	results, err := orbit.ValidatePublishedReferenceSuite()
	if err != nil {
		orbit.LogReferenceValidationResults(results)
		return err
	}
	orbit.LogReferenceValidationResults(results)
	return nil
}

func logLocalTLESummary(path string, simulationTime time.Time) {
	if isRemoteDataSource(path) {
		return
	}

	summary, err := satellite.SummarizeTLEFile(path)
	if err != nil {
		logging.Warnf("Failed to inspect TLE source %s: %v", path, err)
		return
	}

	satellite.LogTLESummary(summary, path)
	satellite.WarnIfSimulationTimeFarFromTLE(summary, simulationTime, path)
}

func runController(simulationController types.SimulationController, simulationConfig configs.SimulationConfig) {
	defer simulationController.Close()

	if simulationConfig.StepInterval >= 0 {
		done := simulationController.StartAutorun()
		<-done
		return
	}

	logging.Infof("Simulation loaded. Not autorunning as StepInterval < 0.")
	const manualStepSeconds = 60 * 10
	for step := 0; step < simulationConfig.StepCount; step++ {
		simulationController.StepBySeconds(manualStepSeconds)
		grounds := simulationController.GetGroundStations()
		ground1 := grounds[0]
		ground2 := grounds[80]
		link1 := bestLink(ground1)
		link2 := bestLink(ground2)
		if link1 == nil || link2 == nil {
			logging.Warnf("No uplink available")
			continue
		}

		uplinkSat1 := link1.GetOther(ground1)
		uplinkSat2 := link2.GetOther(ground2)
		route, err := ground1.GetRouter().RouteToNode(ground2, nil)
		interSatelliteRoute, interSatelliteErr := uplinkSat1.GetRouter().RouteToNode(uplinkSat2, nil)
		logging.Debugf("ISL route: %+v", interSatelliteRoute)
		if err != nil {
			logging.Warnf("Routing error: %v", err)
		} else if !route.Reachable() {
			logging.Warnf("No route from %s to %s", ground1.GetName(), ground2.GetName())
		} else {
			logging.Infof("Route from %s to %s in %d ms", ground1.GetName(), ground2.GetName(), route.Latency())
			logging.Infof("Uplink latency %.3f ms", link1.Latency()+link2.Latency())
			if interSatelliteErr != nil {
				logging.Warnf("Inter-satellite routing error: %v", interSatelliteErr)
			} else if interSatelliteRoute == nil || !interSatelliteRoute.Reachable() {
				logging.Warnf("No inter-satellite route between %s and %s", uplinkSat1.GetName(), uplinkSat2.GetName())
			} else {
				logging.Infof("Latency between uplink nodes: %d ms", interSatelliteRoute.Latency())
			}
			logging.Debugf("%s -> %s -> %s -> %s", ground1.GetName(), uplinkSat1.GetName(), uplinkSat2.GetName(), ground2.GetName())
			logging.Debugf("Distances: %.3f -> %.3f -> %.3f", link1.Distance(), uplinkSat1.DistanceTo(uplinkSat2), link2.Distance())
			if interSatelliteRoute != nil && interSatelliteRoute.Reachable() {
				logging.Debugf("Latencies: %.3f -> %d -> %.3f", link1.Latency(), interSatelliteRoute.Latency(), link2.Latency())
			}
			logging.Debugf("Uplink satellites are %.3f km apart", uplinkSat1.DistanceTo(uplinkSat2)/1000)
			logging.Debugf("Uplink satellite positions: %+v %+v", uplinkSat1.GetPosition(), uplinkSat2.GetPosition())
		}
		logging.Debugf("%d ground stations in simulation", len(grounds))
		logging.Infof("Simulation stepped by %d seconds", manualStepSeconds)

		if sunState, ok := types.FindStatePlugin[stateplugin.SunStatePlugin](simulationController.GetStatePluginRepository()); ok {
			logging.Debugf("Sunlight exposure of %s is %.3f", uplinkSat1.GetName(), sunState.GetSunlightExposure(uplinkSat1))
		}
	}
}

func bestLink(node types.Node) types.Link {
	links := node.GetLinkNodeProtocol().Established()
	if len(links) == 0 {
		return nil
	}

	best := links[0]
	for _, l := range links[1:] {
		if l.Latency() < best.Latency() {
			best = l
		}
	}

	return best
}

func parseListFlag(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func mustBuildSimulationPlugins(pluginNames []string, topologyOutputFile string, runtimeOutputFile string) []types.SimulationPlugin {
	builder := simplugin.NewPluginBuilder().
		WithTopologyOutputFile(topologyOutputFile).
		WithRuntimeOutputFile(runtimeOutputFile)
	plugins, err := builder.BuildPlugins(pluginNames)
	if err != nil {
		logging.Fatalf("Failed to build simulation plugins: %v", err)
	}
	return plugins
}

func mustBuildStatePlugins(pluginNames []string) []types.StatePlugin {
	builder := stateplugin.NewStatePluginBuilder()
	plugins, err := builder.BuildPlugins(pluginNames)
	if err != nil {
		logging.Fatalf("Failed to build state plugins: %v", err)
	}
	return plugins
}

// resourcePath resolves simulator resource names relative to the bundled resources tree.
func resourcePath(kind, name string) string {
	return fmt.Sprintf("./resources/%s/%s", kind, name)
}

func resolveDataSourcePath(kind, value string) string {
	if value == "" {
		return resourcePath(kind, value)
	}
	if isRemoteDataSource(value) || filepath.IsAbs(value) || strings.HasPrefix(value, ".") || strings.Contains(value, "/") {
		return value
	}
	return resourcePath(kind, value)
}

func defaultDownloadedTLEPath(group string, downloadedAt time.Time) string {
	return filepath.Join(".", "resources", "tle", satellite.CurrentTLEFilename(group, downloadedAt))
}

func isRemoteDataSource(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
