package plugin

import (
	"context"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/stretchr/testify/require"
)

func TestPluginNameIsTheSingleName(t *testing.T) {
	require.Equal(t, "vantage", PluginName)

	p := New("test")
	require.Equal(t, PluginName, p.Name())
	require.Equal(t, PluginName, p.Info().Name)

	client := serveTestPlugin(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name, err := client.Name(ctx, &pbc.NameRequest{})
	require.NoError(t, err)
	require.Equal(t, PluginName, name.GetName())

	info, err := client.GetPluginInfo(ctx, &pbc.GetPluginInfoRequest{})
	require.NoError(t, err)
	require.Equal(t, PluginName, info.GetName())
}
