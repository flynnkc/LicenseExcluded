package main

import (
	"context"
	"encoding/json"
	"fmt"
	"func/pkg/clients"
	"io"

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
	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		sendError(out, fmt.Sprintf("Error getting Resource Principal provider: %v", err))
		return
	}

	c, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		sendError(out, fmt.Sprintf("Error getting Identity client: %v", err))
		return
	}

	tenantOcid, err := provider.TenancyOCID()
	if err != nil {
		sendError(out, fmt.Sprintf("Error getting tenant OCID: %v", err))
		return
	}

	regions, err := c.ListRegionSubscriptions(
		context.Background(),
		identity.ListRegionSubscriptionsRequest{
			TenancyId: &tenantOcid,
		},
	)
	if err != nil {
		sendError(out, fmt.Sprintf("Error getting regions subscription: %v", err))
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
