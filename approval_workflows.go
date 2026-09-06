package omnisocials

import "context"

// ApprovalWorkflowsService covers /approval-workflows. Workflows are
// configured in the OmniSocials dashboard (Approvals); the API lists them so
// a post can be routed through one at create time via
// PostCreateParams.ApprovalWorkflowID.
type ApprovalWorkflowsService struct {
	client *Client
}

// ApprovalWorkflowApprover is one approver on a workflow step.
type ApprovalWorkflowApprover struct {
	// ID is the approver's user id.
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// ApprovalWorkflowStep is one step of a workflow; steps are approved in
// order.
type ApprovalWorkflowStep struct {
	// Order is the 1-based step order.
	Order int    `json:"order"`
	Name  string `json:"name"`
	// RequireMode is "any" (one approver of the step is enough) or "all"
	// (every approver must approve).
	RequireMode string                     `json:"require_mode"`
	Approvers   []ApprovalWorkflowApprover `json:"approvers"`
}

// ApprovalWorkflow is a saved approval workflow.
type ApprovalWorkflow struct {
	// ID is what PostCreateParams.ApprovalWorkflowID takes.
	ID   string `json:"id"`
	Name string `json:"name"`
	// WorkspaceID is the workspace the workflow is bound to, or nil when it
	// is available to every workspace of the company.
	WorkspaceID *int                   `json:"workspace_id"`
	Steps       []ApprovalWorkflowStep `json:"steps"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

// List calls `GET /approval-workflows`: the workflows this workspace can use
// (company-wide plus workspace-bound), with steps and named approvers.
func (s *ApprovalWorkflowsService) List(ctx context.Context) (*ListResponse[ApprovalWorkflow], error) {
	var out ListResponse[ApprovalWorkflow]
	if err := s.client.get(ctx, "/approval-workflows", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
