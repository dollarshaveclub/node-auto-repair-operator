package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// EC2Client defines an interface that can send EC2 restart API
// requests to AWS.
type EC2Client interface {
	RebootInstances(*ec2.RebootInstancesInput) (*ec2.RebootInstancesOutput, error)
}

// InstanceRebooter can reboot Kubernetes nodes.
type InstanceRebooter struct {
	ec2Client EC2Client
}

// NewInstanceRebooter instantiates a new InstanceRebooter.
func NewInstanceRebooter(ec2Client EC2Client) *InstanceRebooter {
	return &InstanceRebooter{
		ec2Client: ec2Client,
	}
}

// Reboot restarts a Kubernetes node represented by a *naro.Node.
func (i *InstanceRebooter) Reboot(ctx context.Context, node *naro.Node) error {
	ec2ID := node.Source.Spec.ExternalID

	logrus.Infof("restarting EC2 instance: %s", ec2ID)

	input := &ec2.RebootInstancesInput{
		InstanceIds: []*string{aws.String(ec2ID)},
	}
	output, err := i.ec2Client.RebootInstances(input)
	if err != nil {
		return errors.Wrapf(err, "error rebooting EC2 instance: %s", ec2ID)
	}

	logrus.Infof("successfully rebooted EC2 instance %s: %s", ec2ID, output)

	return nil
}
