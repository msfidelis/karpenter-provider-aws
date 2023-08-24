/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeclass_test

import (
	"context"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-core/pkg/operator/scheme"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"

	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/test"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
}

var ctx context.Context
var env *coretest.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeClaimUtils")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClassUtils", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	BeforeEach(func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily:       aws.String(v1alpha1.AMIFamilyAL2),
				Context:         aws.String("context-1"),
				InstanceProfile: aws.String("profile-1"),
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				SubnetSelector: map[string]string{
					"test-subnet-key": "test-subnet-value",
				},
				SecurityGroupSelector: map[string]string{
					"test-security-group-key": "test-security-group-value",
				},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					MetadataOptions: &v1alpha1.MetadataOptions{
						HTTPEndpoint: aws.String("test-metadata-1"),
					},
					BlockDeviceMappings: []*v1alpha1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
						},
						{
							DeviceName: aws.String("map-device-2"),
						},
					},
				},
			},
			UserData:           aws.String("userdata-test-1"),
			DetailedMonitoring: aws.Bool(false),
			AMISelector: map[string]string{
				"test-ami-key": "test-ami-value",
			},
		})
		nodeTemplate.Status = v1alpha1.AWSNodeTemplateStatus{
			Subnets: []v1alpha1.Subnet{
				{
					ID:   "test-subnet-id",
					Zone: "test-zone-1a",
				},
				{
					ID:   "test-subnet-id2",
					Zone: "test-zone-1b",
				},
			},
			SecurityGroups: []v1alpha1.SecurityGroup{
				{
					ID:   "test-security-group-id",
					Name: "test-security-group-name",
				},
				{
					ID:   "test-security-group-id2",
					Name: "test-security-group-name2",
				},
			},
			AMIs: []v1alpha1.AMI{
				{
					ID:   "test-ami-id",
					Name: "test-ami-name",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
				{
					ID:   "test-ami-id2",
					Name: "test-ami-name2",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"arm64"},
						},
					},
				},
			},
		}
	})
	It("should convert a AWSNodeTemplate to a NodeClass", func() {
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.AMISelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.AMISelector))
		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeNil())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a NodeClass (with AMISelector name and owner values set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::name":   "ami-name1,ami-name2",
			"aws::owners": "self,amazon,123456789",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(6))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("self"),
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("amazon"),
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("123456789"),
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("self"),
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("amazon"),
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("123456789"),
				Tags:  map[string]string{},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeNil())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a NodeClass (with AMISelector id set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::ids": "ami-1234,ami-5678,ami-custom-id",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(3))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				ID:   lo.ToPtr("ami-1234"),
				Tags: map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				ID:   lo.ToPtr("ami-5678"),
				Tags: map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				ID:   lo.ToPtr("ami-custom-id"),
				Tags: map[string]string{},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeNil())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a NodeClass (with AMISelector name, owner, id, and tags set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::name":   "ami-name1,ami-name2",
			"aws::owners": "self,amazon",
			"aws::ids":    "ami-1234,ami-5678",
			"custom-tag":  "custom-value",
			"custom-tag2": "custom-value2",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		// This should include all permutations of the filters that could be used by this selector mechanism
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(8))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("self"),
				ID:    lo.ToPtr("ami-1234"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("self"),
				ID:    lo.ToPtr("ami-5678"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("amazon"),
				ID:    lo.ToPtr("ami-1234"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name1"),
				Owner: lo.ToPtr("amazon"),
				ID:    lo.ToPtr("ami-5678"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("self"),
				ID:    lo.ToPtr("ami-1234"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("self"),
				ID:    lo.ToPtr("ami-5678"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("amazon"),
				ID:    lo.ToPtr("ami-1234"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  lo.ToPtr("ami-name2"),
				Owner: lo.ToPtr("amazon"),
				ID:    lo.ToPtr("ami-5678"),
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeNil())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should retrieve a NodeClass with a get call", func() {
		nodeClass := test.NodeClass()
		ExpectApplied(ctx, env.Client, nodeClass)

		retrieved, err := nodeclassutil.Get(ctx, env.Client, nodeclassutil.Key{Name: nodeClass.Name, IsNodeTemplate: false})
		Expect(err).ToNot(HaveOccurred())
		Expect(retrieved.Name).To(Equal(nodeClass.Name))
	})
	It("should retrieve a AWSNodeTemplate with a get call", func() {
		nodeTemplate := test.AWSNodeTemplate()
		ExpectApplied(ctx, env.Client, nodeTemplate)

		retrieved, err := nodeclassutil.Get(ctx, env.Client, nodeclassutil.Key{Name: nodeTemplate.Name, IsNodeTemplate: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(retrieved.Name).To(Equal(nodeTemplate.Name))
	})
})

func ExpectBlockDeviceMappingsEqual(bdm1 []*v1alpha1.BlockDeviceMapping, bdm2 []*v1beta1.BlockDeviceMapping) {
	// Expect that all BlockDeviceMappings are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(bdm1).To(HaveLen(len(bdm2)))
	sort.Slice(bdm1, func(i, j int) bool {
		return lo.FromPtr(bdm1[i].DeviceName) < lo.FromPtr(bdm1[j].DeviceName)
	})
	sort.Slice(bdm2, func(i, j int) bool {
		return lo.FromPtr(bdm2[i].DeviceName) < lo.FromPtr(bdm2[j].DeviceName)
	})
	for i := range bdm1 {
		Expect(lo.FromPtr(bdm1[i].DeviceName)).To(Equal(lo.FromPtr(bdm2[i].DeviceName)))
		ExpectBlockDevicesEqual(bdm1[i].EBS, bdm2[i].EBS)
	}
}

func ExpectBlockDevicesEqual(bd1 *v1alpha1.BlockDevice, bd2 *v1beta1.BlockDevice) {
	Expect(bd1 == nil).To(Equal(bd2 == nil))
	if bd1 != nil {
		Expect(lo.FromPtr(bd1.DeleteOnTermination)).To(Equal(lo.FromPtr(bd2.VolumeType)))
		Expect(lo.FromPtr(bd1.Encrypted)).To(Equal(lo.FromPtr(bd2.Encrypted)))
		Expect(lo.FromPtr(bd1.IOPS)).To(Equal(lo.FromPtr(bd2.IOPS)))
		Expect(lo.FromPtr(bd1.KMSKeyID)).To(Equal(lo.FromPtr(bd2.KMSKeyID)))
		Expect(lo.FromPtr(bd1.SnapshotID)).To(Equal(lo.FromPtr(bd2.SnapshotID)))
		Expect(lo.FromPtr(bd1.Throughput)).To(Equal(lo.FromPtr(bd2.Throughput)))
		Expect(lo.FromPtr(bd1.VolumeSize)).To(Equal(lo.FromPtr(bd2.VolumeSize)))
		Expect(lo.FromPtr(bd1.VolumeType)).To(Equal(lo.FromPtr(bd2.VolumeType)))
	}
}

func ExpectMetadataOptionsEqual(mo1 *v1alpha1.MetadataOptions, mo2 *v1beta1.MetadataOptions) {
	Expect(mo1 == nil).To(Equal(mo2 == nil))
	if mo1 != nil {
		Expect(lo.FromPtr(mo1.HTTPEndpoint)).To(Equal(lo.FromPtr(mo2.HTTPEndpoint)))
		Expect(lo.FromPtr(mo1.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(mo2.HTTPProtocolIPv6)))
		Expect(lo.FromPtr(mo1.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(mo2.HTTPPutResponseHopLimit)))
		Expect(lo.FromPtr(mo1.HTTPTokens)).To(Equal(lo.FromPtr(mo2.HTTPTokens)))
	}
}

func ExpectSubnetStatusEqual(subnets1 []v1alpha1.Subnet, subnets2 []v1beta1.Subnet) {
	// Expect that all Subnet Status entries are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(subnets1).To(HaveLen(len(subnets2)))
	sort.Slice(subnets1, func(i, j int) bool {
		return subnets1[i].ID < subnets1[j].ID
	})
	sort.Slice(subnets2, func(i, j int) bool {
		return subnets2[i].ID < subnets2[j].ID
	})
	for i := range subnets1 {
		Expect(subnets1[i].ID).To(Equal(subnets2[i].ID))
		Expect(subnets1[i].Zone).To(Equal(subnets2[i].Zone))
	}
}

func ExpectSecurityGroupStatusEqual(securityGroups1 []v1alpha1.SecurityGroup, securityGroups2 []v1beta1.SecurityGroup) {
	// Expect that all SecurityGroup Status entries are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(securityGroups1).To(HaveLen(len(securityGroups2)))
	sort.Slice(securityGroups1, func(i, j int) bool {
		return securityGroups1[i].ID < securityGroups1[j].ID
	})
	sort.Slice(securityGroups2, func(i, j int) bool {
		return securityGroups2[i].ID < securityGroups2[j].ID
	})
	for i := range securityGroups1 {
		Expect(securityGroups1[i].ID).To(Equal(securityGroups2[i].ID))
		Expect(securityGroups1[i].Name).To(Equal(securityGroups2[i].Name))
	}
}

func ExpectAMIStatusEqual(amis1 []v1alpha1.AMI, amis2 []v1beta1.AMI) {
	// Expect that all AMI Status entries are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(amis1).To(HaveLen(len(amis2)))
	sort.Slice(amis1, func(i, j int) bool {
		return amis1[i].ID < amis1[j].ID
	})
	sort.Slice(amis2, func(i, j int) bool {
		return amis2[i].ID < amis2[j].ID
	})
	for i := range amis1 {
		Expect(amis1[i].ID).To(Equal(amis2[i].ID))
		Expect(amis1[i].Name).To(Equal(amis2[i].Name))
		Expect(amis1[i].Requirements).To(ConsistOf(lo.Map(amis2[i].Requirements, func(r v1.NodeSelectorRequirement, _ int) interface{} { return BeEquivalentTo(r) })...))
	}
}
