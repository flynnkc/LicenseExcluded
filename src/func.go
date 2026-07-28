package main

import (
	"context"
	"encoding/json"
	"fmt"
	"func/pkg/clients"
	"func/pkg/logging"
	"func/pkg/results"
	"io"
	"os"

	"github.com/fnproject/fdk-go"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(myHandler))
}

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
	logger := logging.NewLogger(os.Getenv("LOG_LEVEL"))

	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		s := fmt.Sprintf("Error getting Resource Principal provider: %v", err)
		logger.Error(s)
		sendError(out, s)
		return
	}

	c, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		s := fmt.Sprintf("Error getting Identity client: %v", err)
		logger.Error(s)
		sendError(out, s)
		return
	}

	tenantOcid, err := provider.TenancyOCID()
	if err != nil {
		s := fmt.Sprintf("Error getting tenant OCID: %v", err)
		logger.Error(s)
		sendError(out, s)
		return
	}

	regions, err := c.ListRegionSubscriptions(
		ctx,
		identity.ListRegionSubscriptionsRequest{
			TenancyId: &tenantOcid,
		},
	)
	if err != nil {
		s := fmt.Sprintf("Error getting regions subscription: %v", err)
		logger.Error(s)
		sendError(out, s)
		return
	}

	bundle, errs := clients.NewClientBundle(regions.Items)
	msg := bundle.ProcessCollection(ctx)
	for _, err := range errs {
		logger.Error("client bundle setup failed", "error", err)
		msg.AddFailures(1)
	}
	logger.Info("invoke complete", "result", msg.JsonEncode())

	if err := json.NewEncoder(out).Encode(msg); err != nil {
		logger.Error("failed to write response", "error", err)
	}
}

func sendError(out io.Writer, message string) {
	msg := results.Result{Error: message}

	if err := json.NewEncoder(out).Encode(&msg); err != nil {
		logging.NewLogger(os.Getenv("LOG_LEVEL")).Error("failed to write error response", "error", err)
	}
}
