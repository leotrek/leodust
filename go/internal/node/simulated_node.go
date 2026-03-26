package node

import (
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

var _ PrecomputedNode = (*PrecomputedSatellite)(nil)
var _ PrecomputedNode = (*PrecomputedGroundStation)(nil)

type PrecomputedNode interface {
	types.Node
	AddPositionState(time time.Time, position types.Vector)
}
