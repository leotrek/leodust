package satellite

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

type constellationTestLoader struct {
	satellites []types.Satellite
}

func (l *constellationTestLoader) Load(io.Reader) ([]types.Satellite, error) {
	return l.satellites, nil
}

type constellationTestProtocol struct {
	links []types.Link
}

func (p *constellationTestProtocol) Mount(types.Node)                   {}
func (p *constellationTestProtocol) AddLink(link types.Link)            { p.links = append(p.links, link) }
func (p *constellationTestProtocol) ConnectLink(types.Link) error       { return nil }
func (p *constellationTestProtocol) DisconnectLink(types.Link) error    { return nil }
func (p *constellationTestProtocol) UpdateLinks() ([]types.Link, error) { return nil, nil }
func (p *constellationTestProtocol) Established() []types.Link          { return p.links }
func (p *constellationTestProtocol) Links() []types.Link                { return p.links }

type constellationTestSatellite struct {
	name     string
	protocol *constellationTestProtocol
}

func newConstellationTestSatellite(name string) *constellationTestSatellite {
	return &constellationTestSatellite{name: name, protocol: &constellationTestProtocol{}}
}

func (s *constellationTestSatellite) GetName() string                             { return s.name }
func (s *constellationTestSatellite) GetRouter() types.Router                     { return nil }
func (s *constellationTestSatellite) GetComputing() types.Computing               { return nil }
func (s *constellationTestSatellite) GetPosition() types.Vector                   { return types.Vector{} }
func (s *constellationTestSatellite) DistanceTo(other types.Node) float64         { return 0 }
func (s *constellationTestSatellite) UpdatePosition(time.Time)                    {}
func (s *constellationTestSatellite) GetLinkNodeProtocol() types.LinkNodeProtocol { return s.protocol }
func (s *constellationTestSatellite) GetISLProtocol() types.InterSatelliteLinkProtocol {
	return s.protocol
}

func TestLoadSatelliteConstellationBuildsPairwiseCandidateLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constellation.fake")
	if err := os.WriteFile(path, []byte("ignored"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	satA := newConstellationTestSatellite("A")
	satB := newConstellationTestSatellite("B")
	satC := newConstellationTestSatellite("C")

	loader := NewSatelliteConstellationLoader()
	loader.RegisterDataSourceLoader("fake", &constellationTestLoader{
		satellites: []types.Satellite{satA, satB, satC},
	})

	satellites, err := loader.LoadSatelliteConstellation(path, "fake")
	if err != nil {
		t.Fatalf("LoadSatelliteConstellation returned error: %v", err)
	}
	if len(satellites) != 3 {
		t.Fatalf("expected three satellites, got %d", len(satellites))
	}
	if len(satA.protocol.links) != 2 || len(satB.protocol.links) != 2 || len(satC.protocol.links) != 2 {
		t.Fatalf("expected complete-graph candidate links, got counts A=%d B=%d C=%d", len(satA.protocol.links), len(satB.protocol.links), len(satC.protocol.links))
	}
}

func TestLoadSatelliteConstellationRejectsUnsupportedSourceType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constellation.fake")
	if err := os.WriteFile(path, []byte("ignored"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	loader := NewSatelliteConstellationLoader()
	if _, err := loader.LoadSatelliteConstellation(path, "missing"); err == nil {
		t.Fatal("expected unsupported source type to return an error")
	}
}
