package infra

type GarageAdminClient struct {
	*GarageClient
}

func InitGarageAdminClient(garageClient *GarageClient) *GarageAdminClient {
	if garageClient == nil {
		return nil
	}

	return &GarageAdminClient{
		GarageClient: garageClient,
	}
}
