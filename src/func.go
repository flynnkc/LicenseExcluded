package main

import (
	"context"
	"encoding/json"
	"fmt"
	"func/pkg/clients"
	"func/pkg/logging"
	"io"
	"os"

	"github.com/fnproject/fdk-go"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type Message struct {
	Msg string `json:"message"`
}

func main() {
	fdk.Handle(fdk.HandlerFunc(myHandler))
}

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
	logger := logging.NewLogger(os.Getenv("LOG_LEVEL"))

	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		s := fmt.Sprintf("Error getting Resource Principal provider: %v", err)
		logger.Critical(s)
		sendError(out, s)
		return
	}

	c, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		s := fmt.Sprintf("Error getting Identity client: %v", err)
		logger.Critical(s)
		sendError(out, s)
		return
	}

	tenantOcid, err := provider.TenancyOCID()
	if err != nil {
		s := fmt.Sprintf("Error getting tenant OCID: %v", err)
		logger.Critical(s)
		sendError(out, s)
		return
	}

	regions, err := c.ListRegionSubscriptions(
		context.Background(),
		identity.ListRegionSubscriptionsRequest{
			TenancyId: &tenantOcid,
		},
	)
	if err != nil {
		s := fmt.Sprintf("Error getting regions subscription: %v", err)
		logger.Critical(s)
		sendError(out, s)
		return
	}

	clients.NewClientBundle(provider, regions.Items).ProcessCollection()

	msg := Message{
		Msg: "LicenseExcluded invoke complete",
	}
	json.NewEncoder(out).Encode(&msg)
}

func sendError(out io.Writer, message string) {
	msg := Message{Msg: message}

	json.NewEncoder(out).Encode(&msg)
}
