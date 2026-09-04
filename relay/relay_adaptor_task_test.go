package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/advancedcustom"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptorSupportsAdvancedCustomChannels(t *testing.T) {
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeAdvancedCustom))
	require.IsType(t, &advancedcustom.TaskAdaptor{}, GetTaskAdaptor(platform))
}
