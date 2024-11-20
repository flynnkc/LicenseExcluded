package main

import (
	"context"
	clients "func/pkg/clients"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/example/helpers"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

func main() {
	//fdk.Handle(fdk.HandlerFunc(myHandler))
	provider := common.DefaultConfigProvider()
	c, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	helpers.FatalIfError(err)

	tenantOcid, err := provider.TenancyOCID()
	helpers.FatalIfError(err)

	response, err := c.ListRegionSubscriptions(
		context.Background(),
		identity.ListRegionSubscriptionsRequest{
			TenancyId: &tenantOcid,
		},
	)
	helpers.FatalIfError(err)

	bundle := clients.NewClientBundle(provider, response.Items)

	bundle.ProcessCollection()
}

/*
type Person struct {
	Name string `json:"name"`
}

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
	p := &Person{Name: "World"}
	json.NewDecoder(in).Decode(p)
	msg := struct {
		Msg string `json:"message"`
	}{
		Msg: fmt.Sprintf("Hello %s", p.Name),
	}
	log.Print("Inside Go Hello World function")
	json.NewEncoder(out).Encode(&msg)
}
*/
