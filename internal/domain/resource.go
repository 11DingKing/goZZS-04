package domain

import (
	"fmt"
	"sort"
	"time"
)

// ResourceType identifies a piece of emergency equipment.
type ResourceType string

const (
	ResourceTypeDrone   ResourceType = "drone"
	ResourceTypeVehicle ResourceType = "vehicle"
)

// ResourceStatus indicates whether a resource unit is available for dispatch.
type ResourceStatus string

const (
	ResourceStatusAvailable ResourceStatus = "available"
	ResourceStatusInUse     ResourceStatus = "in_use"
)

// Resource is an emergency asset such as a drone or patrol vehicle.
type Resource struct {
	ID       string         `json:"id"`
	Type     ResourceType   `json:"type"`
	Name     string         `json:"name"`
	Capacity int            `json:"capacity"`
	InUse    int            `json:"in_use"`
	Status   ResourceStatus `json:"status"`
}

// Available returns the number of units still free for allocation.
func (r *Resource) Available() int {
	return r.Capacity - r.InUse
}

// ResourceRequestStatus tracks a single resource allocation request.
type ResourceRequestStatus string

const (
	ResourceRequestPending   ResourceRequestStatus = "pending"
	ResourceRequestApproved  ResourceRequestStatus = "approved"
	ResourceRequestAllocated ResourceRequestStatus = "allocated"
	ResourceRequestRejected  ResourceRequestStatus = "rejected"
	ResourceRequestReleased  ResourceRequestStatus = "released"
)

// ResourceRequest is a patrol group's request for an emergency resource
// tied to a specific incident.
type ResourceRequest struct {
	ID             string                `json:"id"`
	IncidentID     string                `json:"incident_id"`
	ResourceID     string                `json:"resource_id"`
	Quantity       int                   `json:"quantity"`
	Priority       Priority              `json:"priority"`
	RequestedAt    time.Time             `json:"requested_at"`
	RequestedBy    string                `json:"requested_by"`
	Status         ResourceRequestStatus `json:"status"`
	ReviewedBy     string                `json:"reviewed_by,omitempty"`
	AllocatedAt    *time.Time            `json:"allocated_at,omitempty"`
	RejectedReason string                `json:"rejected_reason,omitempty"`
}

// Approve transitions a pending request to approved after dispatcher review.
func (rr *ResourceRequest) Approve(dispatcherID string) error {
	if rr.Status != ResourceRequestPending {
		return fmt.Errorf("cannot approve: request status is %s, expected %s", rr.Status, ResourceRequestPending)
	}
	rr.Status = ResourceRequestApproved
	rr.ReviewedBy = dispatcherID
	return nil
}

// Allocate commits the resource to this request and decrements available capacity.
func (rr *ResourceRequest) Allocate(r *Resource, now time.Time) error {
	if rr.Status != ResourceRequestApproved {
		return fmt.Errorf("cannot allocate: request status is %s, expected %s", rr.Status, ResourceRequestApproved)
	}
	if r.Available() < rr.Quantity {
		return fmt.Errorf("insufficient capacity: need %d, available %d", rr.Quantity, r.Available())
	}
	r.InUse += rr.Quantity
	if r.InUse >= r.Capacity {
		r.Status = ResourceStatusInUse
	}
	rr.Status = ResourceRequestAllocated
	rr.AllocatedAt = &now
	return nil
}

// Reject marks a request as rejected with a reason (e.g. over-allocation).
func (rr *ResourceRequest) Reject(reason string) error {
	if rr.Status != ResourceRequestApproved && rr.Status != ResourceRequestPending {
		return fmt.Errorf("cannot reject: request status is %s", rr.Status)
	}
	rr.Status = ResourceRequestRejected
	rr.RejectedReason = reason
	return nil
}

// Release frees the allocated resource units back to the pool.
func (rr *ResourceRequest) Release(r *Resource) error {
	if rr.Status != ResourceRequestAllocated {
		return fmt.Errorf("cannot release: request status is %s, expected %s", rr.Status, ResourceRequestAllocated)
	}
	r.InUse -= rr.Quantity
	if r.InUse < 0 {
		r.InUse = 0
	}
	if r.InUse < r.Capacity {
		r.Status = ResourceStatusAvailable
	}
	rr.Status = ResourceRequestReleased
	return nil
}

// SortRequestsByPriorityThenTime orders requests by descending priority
// then ascending request time, implementing the arbitration rule for
// competing requests on the same resource.
func SortRequestsByPriorityThenTime(reqs []*ResourceRequest) {
	sort.SliceStable(reqs, func(i, j int) bool {
		if reqs[i].Priority != reqs[j].Priority {
			return reqs[i].Priority > reqs[j].Priority
		}
		return reqs[i].RequestedAt.Before(reqs[j].RequestedAt)
	})
}
