// Package main starts the FinFocus Vantage cost source plugin.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus-plugin-vantage/internal/plugin"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "version" {
			_, _ = io.WriteString(os.Stdout, fmt.Sprintf("finfocus-plugin-vantage %s\n", version))
			return
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := pluginsdk.Serve(ctx, pluginsdk.ServeConfig{
		Plugin: plugin.New(version),
		PluginInfo: &pluginsdk.PluginInfo{
			Name: "finfocus-plugin-vantage", Version: version,
			SpecVersion: pluginsdk.SpecVersion,
			Metadata:    map[string]string{"supports_per_request_credentials": "true"},
		},
	})
	stop()
	if err != nil {
		log.Printf("failed to serve: %v", err)
		os.Exit(1)
	}
}
