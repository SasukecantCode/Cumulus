package google

import "context"

type DriveClient struct{}

func NewDriveClient() *DriveClient {
	return &DriveClient{}
}

func (c *DriveClient) Upload(_ context.Context, _ []byte) (string, error) {
	return "", nil
}
