package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openshift/hypershift/support/awsapi"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/go-logr/logr"
	"go.uber.org/mock/gomock"
)

const testInfraID = "test-infra"

// noSuchEntity returns an IAM NoSuchEntityException for use in mock error responses.
func noSuchEntity() *iamtypes.NoSuchEntityException {
	return &iamtypes.NoSuchEntityException{Message: aws.String("no such entity")}
}

// testRole returns an iamtypes.Role for use in mock GetRole responses.
func testRole(name string) *iamtypes.Role {
	return &iamtypes.Role{
		RoleName: aws.String(name),
		Arn:      aws.String("arn:aws:iam::123456789012:role/" + name),
	}
}

// testInstanceProfile returns an iamtypes.InstanceProfile for use in mock responses.
func testInstanceProfile(name string, roles ...iamtypes.Role) *iamtypes.InstanceProfile {
	return &iamtypes.InstanceProfile{
		InstanceProfileName: aws.String(name),
		Roles:               roles,
	}
}

// destroyIAMOptions returns a DestroyIAMOptions with the shared test infraID.
func destroyIAMOptions() *DestroyIAMOptions {
	return &DestroyIAMOptions{
		InfraID: testInfraID,
		Log:     logr.Discard(),
	}
}

func TestDestroyOIDCRole(t *testing.T) {
	// roleName produced by the function under test: "<infraID>-<name>"
	const roleName = testInfraID + "-openshift-ingress"

	tests := []struct {
		name          string
		setupMock     func(*awsapi.MockIAMAPI)
		expectRemoved bool
		expectError   bool
		errorContains string
	}{
		{
			name: "When the role does not exist it should return false without error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
			expectRemoved: false,
		},
		{
			name: "When role exists with no policies it should delete it and return true",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{PolicyNames: []string{}, IsTruncated: false}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
			},
			expectRemoved: true,
		},
		{
			name: "When role exists with managed and inline policies it should detach and delete all then delete the role",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyArn: aws.String("arn:aws:iam::aws:policy/ManagedPolicy"), PolicyName: aws.String("ManagedPolicy")},
						},
					}, nil)
				m.EXPECT().DetachRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DetachRolePolicyOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{PolicyNames: []string{"inline-policy"}, IsTruncated: false}, nil)
				m.EXPECT().DeleteRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRolePolicyOutput{}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
			},
			expectRemoved: true,
		},
		{
			name: "When inline policy delete returns NoSuchEntityException it should ignore and continue",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{PolicyNames: []string{"gone-policy"}, IsTruncated: false}, nil)
				m.EXPECT().DeleteRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
			},
			expectRemoved: true,
		},
		{
			name: "When GetRole returns an API error it should return a wrapped error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("api error"))
			},
			expectError:   true,
			errorContains: "cannot check for existing role",
		},
		{
			name: "When ListAttachedRolePolicies fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("list policies failed"))
			},
			expectError:   true,
			errorContains: "failed to list attached policies",
		},
		{
			name: "When DetachRolePolicy fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyArn: aws.String("arn:aws:iam::aws:policy/ManagedPolicy"), PolicyName: aws.String("ManagedPolicy")},
						},
					}, nil)
				m.EXPECT().DetachRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("detach failed"))
			},
			expectError:   true,
			errorContains: "failed to detach policy",
		},
		{
			name: "When ListRolePolicies fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("list inline failed"))
			},
			expectError:   true,
			errorContains: "failed to list inline policies",
		},
		{
			name: "When DeleteRole fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(roleName)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{PolicyNames: []string{}, IsTruncated: false}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("delete role failed"))
			},
			expectError:   true,
			errorContains: "failed to delete role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockIAM := awsapi.NewMockIAMAPI(ctrl)
			tt.setupMock(mockIAM)

			o := destroyIAMOptions()
			removed, err := o.DestroyOIDCRole(context.Background(), mockIAM, "openshift-ingress")

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				if removed != tt.expectRemoved {
					t.Errorf("expected removed=%v, got %v", tt.expectRemoved, removed)
				}
			}
		})
	}
}

func TestDestroyWorkerInstanceProfile(t *testing.T) {
	const (
		profileName    = testInfraID + "-worker"
		standardRole   = testInfraID + "-worker-role"
		standardPolicy = testInfraID + "-worker-policy"
		rosaRole       = testInfraID + "-worker-" + ROSAWorkerRoleNameSuffix
	)

	tests := []struct {
		name          string
		setupMock     func(*awsapi.MockIAMAPI)
		expectError   bool
		errorContains string
	}{
		{
			name: "When no instance profile or worker roles exist it should return nil",
			setupMock: func(m *awsapi.MockIAMAPI) {
				// existingInstanceProfile
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// existingRole for standard role
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// existingRole for ROSA role
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
		},
		{
			name: "When instance profile with a role exists it should remove the role and delete the profile",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetInstanceProfileOutput{
						InstanceProfile: testInstanceProfile(profileName,
							iamtypes.Role{RoleName: aws.String("worker-role")},
						),
					}, nil)
				m.EXPECT().RemoveRoleFromInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.RemoveRoleFromInstanceProfileOutput{}, nil)
				m.EXPECT().DeleteInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteInstanceProfileOutput{}, nil)
				// standard role and ROSA role not found
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity()).Times(2)
			},
		},
		{
			name: "When standard worker role exists with an inline policy it should delete the policy and the role",
			setupMock: func(m *awsapi.MockIAMAPI) {
				// no instance profile
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// standard role exists
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(standardRole)}, nil)
				// policy exists
				m.EXPECT().GetRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRolePolicyOutput{
						RoleName:   aws.String(standardRole),
						PolicyName: aws.String(standardPolicy),
					}, nil)
				m.EXPECT().DeleteRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRolePolicyOutput{}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
				// ROSA role not found
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
		},
		{
			name: "When ROSA worker role exists with a managed policy it should detach the policy and delete the role",
			setupMock: func(m *awsapi.MockIAMAPI) {
				// no instance profile
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// standard role not found
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// ROSA role exists
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(rosaRole)}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyArn: aws.String("arn:aws:iam::aws:policy/ROSAPolicy"), PolicyName: aws.String("ROSAPolicy")},
						},
					}, nil)
				m.EXPECT().DetachRolePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DetachRolePolicyOutput{}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
			},
		},
		{
			name: "When GetInstanceProfile returns an API error it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("api error"))
			},
			expectError:   true,
			errorContains: "cannot check for existing instance profile",
		},
		{
			name: "When RemoveRoleFromInstanceProfile fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetInstanceProfileOutput{
						InstanceProfile: testInstanceProfile(profileName,
							iamtypes.Role{RoleName: aws.String("worker-role")},
						),
					}, nil)
				m.EXPECT().RemoveRoleFromInstanceProfile(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("remove failed"))
			},
			expectError:   true,
			errorContains: "cannot remove role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockIAM := awsapi.NewMockIAMAPI(ctrl)
			tt.setupMock(mockIAM)

			o := destroyIAMOptions()
			err := o.DestroyWorkerInstanceProfile(context.Background(), mockIAM)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestDestroyOIDCResources(t *testing.T) {
	// OIDC provider ARN whose last path segment matches testInfraID
	const matchingARN = "arn:aws:iam::123456789012:oidc-provider/s3.example.com/" + testInfraID
	const otherARN = "arn:aws:iam::123456789012:oidc-provider/s3.example.com/other-infra"

	tests := []struct {
		name          string
		setupMock     func(*awsapi.MockIAMAPI)
		expectError   bool
		errorContains string
	}{
		{
			name: "When OIDC provider matches infraID and shared role exists it should delete the provider and return early",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().ListOpenIDConnectProviders(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListOpenIDConnectProvidersOutput{
						OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
							{Arn: aws.String(matchingARN)},
						},
					}, nil)
				m.EXPECT().DeleteOpenIDConnectProvider(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteOpenIDConnectProviderOutput{}, nil)
				// DestroyOIDCRole("shared-role") → role found → early return
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.GetRoleOutput{Role: testRole(testInfraID + "-shared-role")}, nil)
				m.EXPECT().ListAttachedRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListAttachedRolePoliciesOutput{}, nil)
				m.EXPECT().ListRolePolicies(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListRolePoliciesOutput{IsTruncated: false}, nil)
				m.EXPECT().DeleteRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.DeleteRoleOutput{}, nil)
			},
		},
		{
			name: "When OIDC provider ARN does not match infraID it should skip deletion and proceed with role cleanup",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().ListOpenIDConnectProviders(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListOpenIDConnectProvidersOutput{
						OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
							{Arn: aws.String(otherARN)},
						},
					}, nil)
				// All role lookups return not found — covers shared-role + 9 component roles
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity()).AnyTimes()
			},
		},
		{
			name: "When DeleteOpenIDConnectProvider returns NoSuchEntityException it should ignore and continue",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().ListOpenIDConnectProviders(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListOpenIDConnectProvidersOutput{
						OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
							{Arn: aws.String(matchingARN)},
						},
					}, nil)
				m.EXPECT().DeleteOpenIDConnectProvider(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// continues to role cleanup; all roles not found
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity()).AnyTimes()
			},
		},
		{
			name: "When ListOpenIDConnectProviders fails it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().ListOpenIDConnectProviders(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("api error"))
			},
			expectError:   true,
			errorContains: "api error",
		},
		{
			name: "When DeleteOpenIDConnectProvider fails with a non-NSE error it should return the error",
			setupMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().ListOpenIDConnectProviders(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&iam.ListOpenIDConnectProvidersOutput{
						OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
							{Arn: aws.String(matchingARN)},
						},
					}, nil)
				m.EXPECT().DeleteOpenIDConnectProvider(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockIAM := awsapi.NewMockIAMAPI(ctrl)
			tt.setupMock(mockIAM)

			o := destroyIAMOptions()
			err := o.DestroyOIDCResources(context.Background(), mockIAM)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestDestroySharedVPCRoles(t *testing.T) {
	tests := []struct {
		name                  string
		privateZonesInCluster bool
		setupIAMMock          func(*awsapi.MockIAMAPI)
		setupVPCOwnerMock     func(*awsapi.MockIAMAPI)
		expectError           bool
		errorContains         string
	}{
		{
			name:                  "When PrivateZonesInClusterAccount is false ingress role should use vpcOwnerClient",
			privateZonesInCluster: false,
			setupIAMMock:          func(_ *awsapi.MockIAMAPI) {},
			setupVPCOwnerMock: func(m *awsapi.MockIAMAPI) {
				// ingress role not found on vpcOwner
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// control-plane role not found on vpcOwner
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
		},
		{
			name:                  "When PrivateZonesInClusterAccount is true ingress role should use iamClient",
			privateZonesInCluster: true,
			setupIAMMock: func(m *awsapi.MockIAMAPI) {
				// ingress role not found on iamClient
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
			setupVPCOwnerMock: func(m *awsapi.MockIAMAPI) {
				// control-plane role not found on vpcOwner
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
			},
		},
		{
			name:                  "When destroying the ingress role fails it should return the error",
			privateZonesInCluster: false,
			setupIAMMock:          func(_ *awsapi.MockIAMAPI) {},
			setupVPCOwnerMock: func(m *awsapi.MockIAMAPI) {
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("api error"))
			},
			expectError:   true,
			errorContains: "cannot check for existing role",
		},
		{
			name:                  "When destroying the control-plane role fails it should return the error",
			privateZonesInCluster: false,
			setupIAMMock:          func(_ *awsapi.MockIAMAPI) {},
			setupVPCOwnerMock: func(m *awsapi.MockIAMAPI) {
				// ingress role not found
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, noSuchEntity())
				// control-plane role returns API error
				m.EXPECT().GetRole(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("api error"))
			},
			expectError:   true,
			errorContains: "cannot check for existing role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockIAM := awsapi.NewMockIAMAPI(ctrl)
			mockVPCOwner := awsapi.NewMockIAMAPI(ctrl)
			tt.setupIAMMock(mockIAM)
			tt.setupVPCOwnerMock(mockVPCOwner)

			o := &DestroyIAMOptions{
				InfraID:                      testInfraID,
				Log:                          logr.Discard(),
				PrivateZonesInClusterAccount: tt.privateZonesInCluster,
			}
			err := o.DestroySharedVPCRoles(context.Background(), mockIAM, mockVPCOwner)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}
