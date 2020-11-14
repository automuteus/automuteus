package discord

import (
	"os"
	"testing"
)

func TestTerminateScalewayNode(t *testing.T) {
	orgId := os.Getenv("SCW_ORGANIZATION_ID")
	accessKey := os.Getenv("SCW_ACCESS_KEY")
	secretKey := os.Getenv("SCW_SECRET_KEY")
	nodeID := os.Getenv("SCW_NODE_ID")

	TerminateScalewayNode(orgId, accessKey, secretKey, nodeID)
}
