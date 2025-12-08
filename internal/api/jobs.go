package api

import "time"

// GetJobs returns all Nomad jobs with their allocations.
func (c *Client) GetJobs() ([]Job, error) {
	// Check if Nomad is healthy first
	if c.getNomadStatus().Status != "healthy" {
		return nil, nil
	}

	var stubs []JobListStub
	if err := c.get(c.nomadAddr+"/v1/jobs", &stubs); err != nil {
		return nil, err
	}

	var jobs []Job
	for _, stub := range stubs {
		job := Job{
			ID:         stub.ID,
			Name:       stub.Name,
			Type:       stub.Type,
			Status:     stub.Status,
			SubmitTime: time.Unix(0, stub.SubmitTime),
		}

		// Get allocations for this job
		allocs, err := c.getJobAllocations(stub.ID)
		if err == nil {
			job.Allocations = allocs
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (c *Client) getJobAllocations(jobID string) ([]Alloc, error) {
	var stubs []AllocListStub
	if err := c.get(c.nomadAddr+"/v1/job/"+jobID+"/allocations", &stubs); err != nil {
		return nil, err
	}

	var allocs []Alloc
	for _, stub := range stubs {
		allocs = append(allocs, Alloc{
			ID:            stub.ID,
			NodeID:        stub.NodeID,
			NodeName:      stub.NodeName,
			TaskGroup:     stub.TaskGroup,
			ClientStatus:  stub.ClientStatus,
			DesiredStatus: stub.DesiredStatus,
		})
	}

	return allocs, nil
}

// GetNodes returns all Nomad client nodes.
func (c *Client) GetNodes() ([]Node, error) {
	// Check if Nomad is healthy first
	if c.getNomadStatus().Status != "healthy" {
		return nil, nil
	}

	var stubs []NodeListStub
	if err := c.get(c.nomadAddr+"/v1/nodes", &stubs); err != nil {
		return nil, err
	}

	var nodes []Node
	for _, stub := range stubs {
		nodes = append(nodes, Node{
			ID:         stub.ID,
			Name:       stub.Name,
			Address:    stub.Address,
			Status:     stub.Status,
			Datacenter: stub.Datacenter,
			NodeClass:  stub.NodeClass,
			Drain:      stub.Drain,
		})
	}

	return nodes, nil
}
