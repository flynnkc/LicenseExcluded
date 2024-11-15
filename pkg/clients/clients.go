package clients

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/analytics"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

const query string = "query database, autonomousdatabase, analyticsinstance resources where lifeCycleState = 'RUNNING' || lifeCycleState = 'STOPPED'"

type RegionalClient struct {
	AnalyticsClient analytics.AnalyticsClient
	DatabaseClient  database.DatabaseClient
	//IdentityClient  identity.IdentityClient
	SearchClient resourcesearch.ResourceSearchClient
}

type ClientBundle map[string]RegionalClient

func NewRegionalClient(p common.ConfigurationProvider) RegionalClient {

	ac, err := analytics.NewAnalyticsClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	dc, err := database.NewDatabaseClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	//ic, err := identity.NewIdentityClientWithConfigurationProvider(p)
	//logErrAndContinue(err)

	sc, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	return RegionalClient{
		AnalyticsClient: ac,
		DatabaseClient:  dc,
		//IdentityClient:  ic,
		SearchClient: sc,
	}
}

// NewClientBundle takes a ConfigurationProvider and a list of regions, creates clients for each region,
// and returns them as a bundle.
func NewClientBundle(p common.ConfigurationProvider, regions []identity.RegionSubscription) ClientBundle {
	cb := make(ClientBundle)

	for _, r := range regions {
		newProvider, err := constructConfigurationProvider(*r.RegionName, p)
		if err != nil {
			fmt.Println("Error creating provider:", err)
			continue
		}

		cb[*r.RegionName] = NewRegionalClient(*newProvider)
	}

	return cb
}

// ProcessCollection fans out on returned resources
func (c *ClientBundle) ProcessCollection() {

	r := make([]resourcesearch.ResourceSummary, 0)

	for _, client := range *c {
		rc := client.Search()
		r = append(r, rc.Items...)
	}

	var wg sync.WaitGroup

	for _, item := range r {
		switch *item.ResourceType {
		case "Database":
			func() {
				wg.Add(1)
				defer wg.Done()
				fmt.Println("database", item)
			}()
		case "AutonomousDatabase":
			func() {
				wg.Add(1)
				defer wg.Done()
				fmt.Println("AutonomousDatabase", item)
			}()
		case "AnalyticsInstance":
			func() {
				wg.Add(1)
				defer wg.Done()
				fmt.Println("AnalyticsInstance", item)
			}()
		default:
			fmt.Println("Error: No supported type", *item.ResourceType)
		}
	}

	wg.Wait()
}

// Search uses the search client to find resources.
func (c *RegionalClient) Search() resourcesearch.ResourceSummaryCollection {

	details := resourcesearch.StructuredSearchDetails{
		Query: common.String(query),
	}

	request := resourcesearch.SearchResourcesRequest{
		SearchDetails: details,
		Limit:         common.Int(1000),
	}

	response, err := c.SearchClient.SearchResources(context.Background(), request)
	logErrAndContinue(err)

	fmt.Printf("Items returned: %v\n", len(response.ResourceSummaryCollection.Items))
	return response.ResourceSummaryCollection
}

func constructConfigurationProvider(region string, provider common.ConfigurationProvider) (*common.ConfigurationProvider, error) {
	tenancy, err := provider.TenancyOCID()
	if err != nil {
		return nil, err
	}

	user, err := provider.UserOCID()
	if err != nil {
		return nil, err
	}

	fingerprint, err := provider.KeyFingerprint()
	if err != nil {
		return nil, err
	}

	passphrase := ""

	/*
		// Get key into string
		key, err := provider.PrivateRSAKey()
		if err != nil {
			return nil, err
		}

		pk := string(x509.MarshalPKCS1PrivateKey(key))
	*/

	pb, err := os.ReadFile(os.Getenv("KC_KEY"))
	logErrAndContinue(err)
	pk := string(pb)

	provider = common.NewRawConfigurationProvider(tenancy, user, region, fingerprint, pk, &passphrase)

	return &provider, nil
}

// Logs error if not nil and moves on without stopping execution
func logErrAndContinue(err error) {
	if err != nil {
		fmt.Println("Error:", err)
	}
}
