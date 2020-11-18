package metrics

import (
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"log"
	"os"
	"strings"
)

func TerminateScalewayNode(org, access, secret, nodeID string) {
	client, err := scw.NewClient(
		// Get your credentials at https://console.scaleway.com/project/credentials
		scw.WithDefaultOrganizationID(org),
		scw.WithAuth(access, secret),
	)
	if err != nil {
		panic(err)
	}

	kApi := k8s.NewAPI(client)

	nodes, err := kApi.ListNodes(&k8s.ListNodesRequest{
		Region:    scw.RegionFrPar,
		ClusterID: os.Getenv("SCW_CLUSTER_ID"),
		PoolID:    nil,
		OrderBy:   "",
		Page:      nil,
		PageSize:  nil,
		Name:      nil,
		Status:    "",
	})
	if err != nil {
		log.Println(err)
		return
	}
	refUid := formatUUID(nodeID)
	log.Println(refUid)
	for _, v := range nodes.Nodes {
		log.Println(v.ID)
		if strings.HasPrefix(v.ID, refUid) {
			nodeID = v.ID
			break
		}
	}

	_, err = kApi.ReplaceNode(&k8s.ReplaceNodeRequest{
		Region: scw.RegionFrPar,
		NodeID: nodeID,
	})
	if err != nil {
		log.Println("Error replacing " + refUid + ": " + err.Error())
	}
}

func formatUUID(id string) string {
	ids := strings.Split(id, "-")
	id = ids[len(ids)-1]
	if len(id) < 20 {
		return ""
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}
