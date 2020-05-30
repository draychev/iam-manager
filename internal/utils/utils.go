package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	iammanagerv1alpha1 "github.com/keikoproj/iam-manager/api/v1alpha1"
	"github.com/keikoproj/iam-manager/internal/config"
	"github.com/keikoproj/iam-manager/pkg/log"
	"strings"
)

//GetTrustPolicy constructs trust policy
func GetTrustPolicy(ctx context.Context, role *iammanagerv1alpha1.Iamrole) (string, error) {
	log := log.Logger(ctx, "internal.utils.utils", "GetTrustPolicy")
	tPolicy := role.Spec.AssumeRolePolicyDocument

	var statements []iammanagerv1alpha1.TrustPolicyStatement

	// Is it IRSA use case
	flag, saName := ParseIRSAAnnotation(ctx, role)

	//Construct AssumeRoleWithWebIdentity
	if flag {

		hostPath := fmt.Sprintf("%s", strings.TrimPrefix(config.Props.OIDCIssuerUrl(), "https://"))
		statement := iammanagerv1alpha1.TrustPolicyStatement{
			Effect: "Allow",
			Action: "sts:AssumeRoleWithWebIdentity",
			Principal: iammanagerv1alpha1.Principal{
				Federated: fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", config.Props.AWSAccountID(), hostPath),
			},
			Condition: &iammanagerv1alpha1.Condition{
				StringEquals: map[string]string{
					fmt.Sprintf("%s:sub", hostPath): fmt.Sprintf("system:serviceaccount:%s:%s", role.ObjectMeta.Namespace, saName),
				},
			},
		}
		statements = append(statements, statement)

	} else {
		// NON - IRSA which should cover AssumeRole usecase
		//For default use cases
		if tPolicy == nil || len(tPolicy.Statement) == 0 {
			if len(config.Props.TrustPolicyARNs()) < 1 {
				msg := "default trust policy is not provided in the config map. Request must provide trust policy in the CR"
				err := errors.New(msg)
				log.Error(err, msg)
				return "", err
			}

			var aws []string
			for _, arn := range config.Props.TrustPolicyARNs() {
				aws = append(aws, arn)
			}

			statement := iammanagerv1alpha1.TrustPolicyStatement{
				Effect: "Allow",
				Action: "sts:AssumeRole",
				Principal: iammanagerv1alpha1.Principal{
					AWS: aws,
				},
			}
			statements = append(statements, statement)
		}
	}

	// If anything included in the request
	if tPolicy != nil && len(tPolicy.Statement) > 0 {
		statements = append(statements, role.Spec.AssumeRolePolicyDocument.Statement...)
	}
	tDoc := iammanagerv1alpha1.AssumeRolePolicyDocument{
		Version:   "2012-10-17",
		Statement: statements,
	}
	//Convert it to string

	output, err := json.Marshal(tDoc)
	if err != nil {
		msg := fmt.Sprintf("malformed trust policy document. unable to marshal it, err = %s", err.Error())
		err := errors.New(msg)
		log.Error(err, msg)
		return "", err
	}
	log.V(1).Info("trust policy generated successfully", "trust_policy", string(output))
	return string(output), nil
}
