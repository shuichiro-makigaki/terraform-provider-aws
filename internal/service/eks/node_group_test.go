package eks_test

import (
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	multierror "github.com/hashicorp/go-multierror"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tfeks "github.com/hashicorp/terraform-provider-aws/internal/service/eks"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func init() {
	acctest.RegisterServiceErrorCheckFunc(eks.EndpointsID, testAccErrorCheckSkipEKS)

	resource.AddTestSweepers("aws_eks_node_group", &resource.Sweeper{
		Name: "aws_eks_node_group",
		F:    testSweepEksNodeGroups,
	})
}

func testSweepEksNodeGroups(region string) error {
	client, err := acctest.SharedRegionalSweeperClient(region)
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}
	conn := client.(*conns.AWSClient).EKSConn
	input := &eks.ListClustersInput{}
	var sweeperErrs *multierror.Error
	sweepResources := make([]*acctest.SweepResource, 0)

	err = conn.ListClustersPages(input, func(page *eks.ListClustersOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, cluster := range page.Clusters {
			input := &eks.ListNodegroupsInput{
				ClusterName: cluster,
			}

			err := conn.ListNodegroupsPages(input, func(page *eks.ListNodegroupsOutput, lastPage bool) bool {
				if page == nil {
					return !lastPage
				}

				for _, nodeGroup := range page.Nodegroups {
					r := ResourceNodeGroup()
					d := r.Data(nil)
					d.SetId(tfeks.NodeGroupCreateResourceID(aws.StringValue(cluster), aws.StringValue(nodeGroup)))

					sweepResources = append(sweepResources, acctest.NewSweepResource(r, d, client))
				}

				return !lastPage
			})

			if acctest.SkipSweepError(err) {
				continue
			}

			if err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing EKS Node Groups (%s): %w", region, err))
			}
		}

		return !lastPage
	})

	if acctest.SkipSweepError(err) {
		log.Printf("[WARN] Skipping EKS Node Groups sweep for %s: %s", region, err)
		return sweeperErrs.ErrorOrNil() // In case we have completed some pages, but had errors
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing EKS Clusters (%s): %w", region, err))
	}

	err = acctest.SweepOrchestrator(sweepResources)

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error sweeping EKS Node Groups (%s): %w", region, err))
	}

	return sweeperErrs.ErrorOrNil()
}

func TestAccAWSEksNodeGroup_basic(t *testing.T) {
	var nodeGroup eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	eksClusterResourceName := "aws_eks_cluster.test"
	iamRoleResourceName := "aws_iam_role.node"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigNodeGroupName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup),
					resource.TestCheckResourceAttr(resourceName, "ami_type", eks.AMITypesAl2X8664),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "eks", regexp.MustCompile(fmt.Sprintf("nodegroup/%[1]s/%[1]s/.+", rName))),
					resource.TestCheckResourceAttrPair(resourceName, "cluster_name", eksClusterResourceName, "name"),
					resource.TestCheckResourceAttr(resourceName, "capacity_type", eks.CapacityTypesOnDemand),
					resource.TestCheckResourceAttr(resourceName, "disk_size", "20"),
					resource.TestCheckResourceAttr(resourceName, "instance_types.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "labels.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "node_group_name", rName),
					resource.TestCheckResourceAttr(resourceName, "node_group_name_prefix", ""),
					resource.TestCheckResourceAttrPair(resourceName, "node_role_arn", iamRoleResourceName, "arn"),
					resource.TestMatchResourceAttr(resourceName, "release_version", regexp.MustCompile(`^\d+\.\d+\.\d+-\d{8}$`)),
					resource.TestCheckResourceAttr(resourceName, "remote_access.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "resources.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "resources.0.autoscaling_groups.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "status", eks.NodegroupStatusActive),
					resource.TestCheckResourceAttr(resourceName, "subnet_ids.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "taint.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "update_config.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "version", eksClusterResourceName, "version"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_Name_Generated(t *testing.T) {
	var nodeGroup eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigNodeGroupNameGenerated(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup),
					create.TestCheckResourceAttrNameGenerated(resourceName, "node_group_name"),
					resource.TestCheckResourceAttr(resourceName, "node_group_name_prefix", "terraform-"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_NamePrefix(t *testing.T) {
	var nodeGroup eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigNodeGroupNamePrefix(rName, "tf-acc-test-prefix-"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup),
					create.TestCheckResourceAttrNameFromPrefix(resourceName, "node_group_name", "tf-acc-test-prefix-"),
					resource.TestCheckResourceAttr(resourceName, "node_group_name_prefix", "tf-acc-test-prefix-"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_disappears(t *testing.T) {
	var nodeGroup eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigNodeGroupName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceNodeGroup(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_AmiType(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigAmiType(rName, eks.AMITypesAl2X8664Gpu),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "ami_type", eks.AMITypesAl2X8664Gpu),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigAmiType(rName, eks.AMITypesAl2Arm64),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "ami_type", eks.AMITypesAl2Arm64),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_CapacityType_Spot(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigCapacityType(rName, eks.CapacityTypesSpot),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "capacity_type", eks.CapacityTypesSpot),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_DiskSize(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigDiskSize(rName, 21),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "disk_size", "21"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ForceUpdateVersion(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigForceUpdateVersion(rName, "1.19"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "version", "1.19"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_update_version"},
			},
			{
				Config: testAccAWSEksNodeGroupConfigForceUpdateVersion(rName, "1.20"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "version", "1.20"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_InstanceTypes_Multiple(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	ec2InstanceTypeOfferingsDataSourceName := "data.aws_ec2_instance_type_offerings.available"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigInstanceTypesMultiple(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttrPair(resourceName, "instance_types.#", ec2InstanceTypeOfferingsDataSourceName, "instance_types.#"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_InstanceTypes_Single(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigInstanceTypesSingle(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "instance_types.#", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_Labels(t *testing.T) {
	var nodeGroup1, nodeGroup2, nodeGroup3 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigLabels1(rName, "key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "labels.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "labels.key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigLabels2(rName, "key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "labels.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "labels.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "labels.key2", "value2"),
				),
			},
			{
				Config: testAccAWSEksNodeGroupConfigLabels1(rName, "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup3),
					resource.TestCheckResourceAttr(resourceName, "labels.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "labels.key2", "value2"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_LaunchTemplate_Id(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	launchTemplateResourceName1 := "aws_launch_template.test1"
	launchTemplateResourceName2 := "aws_launch_template.test2"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateId1(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.id", launchTemplateResourceName1, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateId2(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.id", launchTemplateResourceName2, "id"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_LaunchTemplate_Name(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	launchTemplateResourceName1 := "aws_launch_template.test1"
	launchTemplateResourceName2 := "aws_launch_template.test2"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateName1(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.name", launchTemplateResourceName1, "name"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateName2(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.name", launchTemplateResourceName2, "name"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_LaunchTemplate_Version(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	launchTemplateResourceName := "aws_launch_template.test"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateVersion1(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.version", launchTemplateResourceName, "default_version"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigLaunchTemplateVersion2(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "launch_template.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "launch_template.0.version", launchTemplateResourceName, "default_version"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ReleaseVersion(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	ssmParameterDataSourceName := "data.aws_ssm_parameter.test"
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigReleaseVersion(rName, "1.17"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttrPair(resourceName, "release_version", ssmParameterDataSourceName, "value"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigReleaseVersion(rName, "1.18"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttrPair(resourceName, "release_version", ssmParameterDataSourceName, "value"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_RemoteAccess_Ec2SshKey(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigRemoteAccessEc2SshKey(rName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "remote_access.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "remote_access.0.ec2_ssh_key", rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_RemoteAccess_SourceSecurityGroupIds(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigRemoteAccessSourceSecurityGroupIds1(rName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "remote_access.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "remote_access.0.source_security_group_ids.#", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ScalingConfig_DesiredSize(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 2, 2, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 1, 2, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ScalingConfig_MaxSize(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 1, 2, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 1, 1, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ScalingConfig_MinSize(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 2, 2, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "2"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 2, 2, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_ScalingConfig_Zero_DesiredSize_MinSize(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 0, 1, 0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "0"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 1, 2, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "2"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "1"),
				),
			},
			{
				Config: testAccAWSEksNodeGroupConfigScalingConfigSizes(rName, 0, 1, 0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.desired_size", "0"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.max_size", "1"),
					resource.TestCheckResourceAttr(resourceName, "scaling_config.0.min_size", "0"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_Tags(t *testing.T) {
	var nodeGroup1, nodeGroup2, nodeGroup3 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigTags1(rName, "key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigTags2(rName, "key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
			{
				Config: testAccAWSEksNodeGroupConfigTags1(rName, "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup3),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup2, &nodeGroup3),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_Taints(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigTaints1(rName, "key1", "value1", "NO_SCHEDULE"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "taint.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "taint.*", map[string]string{
						"key":    "key1",
						"value":  "value1",
						"effect": "NO_SCHEDULE",
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigTaints2(rName,
					"key1", "value1updated", "NO_EXECUTE",
					"key2", "value2", "NO_SCHEDULE"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "taint.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "taint.*", map[string]string{
						"key":    "key1",
						"value":  "value1updated",
						"effect": "NO_EXECUTE",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "taint.*", map[string]string{
						"key":    "key2",
						"value":  "value2",
						"effect": "NO_SCHEDULE",
					}),
				),
			},
			{
				Config: testAccAWSEksNodeGroupConfigTaints1(rName, "key2", "value2", "NO_SCHEDULE"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "taint.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "taint.*", map[string]string{
						"key":    "key2",
						"value":  "value2",
						"effect": "NO_SCHEDULE",
					}),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_UpdateConfig(t *testing.T) {
	var nodeGroup1 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigUpdateConfig1(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "update_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "update_config.0.max_unavailable", "1"),
					resource.TestCheckResourceAttr(resourceName, "update_config.0.max_unavailable_percentage", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigUpdateConfig2(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "update_config.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "update_config.0.max_unavailable", "0"),
					resource.TestCheckResourceAttr(resourceName, "update_config.0.max_unavailable_percentage", "40"),
				),
			},
		},
	})
}

func TestAccAWSEksNodeGroup_Version(t *testing.T) {
	var nodeGroup1, nodeGroup2 eks.Nodegroup
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_eks_node_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEks(t) },
		ErrorCheck:   acctest.ErrorCheck(t, eks.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEksNodeGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEksNodeGroupConfigVersion(rName, "1.19"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup1),
					resource.TestCheckResourceAttr(resourceName, "version", "1.19"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEksNodeGroupConfigVersion(rName, "1.20"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEksNodeGroupExists(resourceName, &nodeGroup2),
					testAccCheckAWSEksNodeGroupNotRecreated(&nodeGroup1, &nodeGroup2),
					resource.TestCheckResourceAttr(resourceName, "version", "1.20"),
				),
			},
		},
	})
}

func testAccErrorCheckSkipEKS(t *testing.T) resource.ErrorCheckFunc {
	return acctest.ErrorCheckSkipMessagesContaining(t,
		"InvalidParameterException: The following supplied instance types do not exist",
	)
}

func testAccCheckAWSEksNodeGroupExists(resourceName string, nodeGroup *eks.Nodegroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No EKS Node Group ID is set")
		}

		clusterName, nodeGroupName, err := tfeks.NodeGroupParseResourceID(rs.Primary.ID)

		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).EKSConn

		output, err := tfeks.FindNodegroupByClusterNameAndNodegroupName(conn, clusterName, nodeGroupName)

		if err != nil {
			return err
		}

		*nodeGroup = *output

		return nil
	}
}

func testAccCheckAWSEksNodeGroupDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).EKSConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_eks_node_group" {
			continue
		}

		clusterName, nodeGroupName, err := tfeks.NodeGroupParseResourceID(rs.Primary.ID)

		if err != nil {
			return err
		}

		_, err = tfeks.FindNodegroupByClusterNameAndNodegroupName(conn, clusterName, nodeGroupName)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("EKS Node Group %s still exists", rs.Primary.ID)
	}

	return nil
}

func testAccCheckAWSEksNodeGroupNotRecreated(i, j *eks.Nodegroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if !aws.TimeValue(i.CreatedAt).Equal(aws.TimeValue(j.CreatedAt)) {
			return fmt.Errorf("EKS Node Group (%s) was recreated", aws.StringValue(j.NodegroupName))
		}

		return nil
	}
}

func testAccCheckAWSEksNodeGroupRecreated(i, j *eks.Nodegroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.TimeValue(i.CreatedAt).Equal(aws.TimeValue(j.CreatedAt)) {
			return fmt.Errorf("EKS Node Group (%s) was not recreated", aws.StringValue(j.NodegroupName))
		}

		return nil
	}
}

func testAccAWSEksNodeGroupConfigBaseIamAndVpc(rName string) string {
	return fmt.Sprintf(`
data "aws_availability_zones" "available" {
  state = "available"

  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

data "aws_partition" "current" {}

resource "aws_iam_role" "cluster" {
  name = "%[1]s-cluster"

  assume_role_policy = jsonencode({
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = [
          "eks.${data.aws_partition.current.dns_suffix}",
          "eks-nodegroup.${data.aws_partition.current.dns_suffix}",
        ]
      }
    }]
    Version = "2012-10-17"
  })
}

resource "aws_iam_role_policy_attachment" "cluster-AmazonEKSClusterPolicy" {
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.cluster.name
}

resource "aws_iam_role" "node" {
  name = "%[1]s-node"

  assume_role_policy = jsonencode({
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.${data.aws_partition.current.dns_suffix}"
      }
    }]
    Version = "2012-10-17"
  })
}

resource "aws_iam_role_policy_attachment" "node-AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node.name
}

resource "aws_iam_role_policy_attachment" "node-AmazonEKS_CNI_Policy" {
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.node.name
}

resource "aws_iam_role_policy_attachment" "node-AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.node.name
}

resource "aws_vpc" "test" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name                          = %[1]q
    "kubernetes.io/cluster/%[1]s" = "shared"
  }
}

resource "aws_internet_gateway" "test" {
  vpc_id = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_route_table" "test" {
  vpc_id = aws_vpc.test.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.test.id
  }

  tags = {
    Name = %[1]q
  }
}

resource "aws_main_route_table_association" "test" {
  route_table_id = aws_route_table.test.id
  vpc_id         = aws_vpc.test.id
}

resource "aws_security_group" "test" {
  name   = %[1]q
  vpc_id = aws_vpc.test.id

  egress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = 0
    protocol    = -1
    to_port     = 0
  }

  ingress {
    cidr_blocks = [aws_vpc.test.cidr_block]
    from_port   = 0
    protocol    = -1
    to_port     = 0
  }

  tags = {
    Name = %[1]q
  }
}

resource "aws_subnet" "test" {
  count = 2

  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(aws_vpc.test.cidr_block, 8, count.index)
  map_public_ip_on_launch = true
  vpc_id                  = aws_vpc.test.id

  tags = {
    Name                          = %[1]q
    "kubernetes.io/cluster/%[1]s" = "shared"
  }
}
`, rName)
}

func testAccAWSEksNodeGroupConfigBase(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBaseIamAndVpc(rName),
		fmt.Sprintf(`
resource "aws_eks_cluster" "test" {
  name     = %[1]q
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    subnet_ids = aws_subnet.test[*].id
  }

  depends_on = [
    aws_iam_role_policy_attachment.cluster-AmazonEKSClusterPolicy,
    aws_main_route_table_association.test,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigBaseVersion(rName string, version string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBaseIamAndVpc(rName),
		fmt.Sprintf(`
resource "aws_eks_cluster" "test" {
  name     = %[1]q
  role_arn = aws_iam_role.cluster.arn
  version  = %[2]q

  vpc_config {
    subnet_ids = aws_subnet.test[*].id
  }

  depends_on = [
    aws_iam_role_policy_attachment.cluster-AmazonEKSClusterPolicy,
    aws_main_route_table_association.test,
  ]
}
`, rName, version))
}

func testAccAWSEksNodeGroupConfigNodeGroupName(rName string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigNodeGroupNameGenerated(rName string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), `
resource "aws_eks_node_group" "test" {
  cluster_name  = aws_eks_cluster.test.name
  node_role_arn = aws_iam_role.node.arn
  subnet_ids    = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`)
}

func testAccAWSEksNodeGroupConfigNodeGroupNamePrefix(rName, namePrefix string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name           = aws_eks_cluster.test.name
  node_group_name_prefix = %[1]q
  node_role_arn          = aws_iam_role.node.arn
  subnet_ids             = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    "aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy",
    "aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy",
    "aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly",
  ]
}
`, namePrefix))
}

func testAccAWSEksNodeGroupConfigAmiType(rName, amiType string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  ami_type        = %[2]q
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, amiType))
}

func testAccAWSEksNodeGroupConfigCapacityType(rName, capacityType string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  capacity_type   = %[2]q
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, capacityType))
}

func testAccAWSEksNodeGroupConfigDiskSize(rName string, diskSize int) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  disk_size       = %[2]d
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, diskSize))
}

func testAccAWSEksNodeGroupConfigForceUpdateVersion(rName, version string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBaseVersion(rName, version), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name         = aws_eks_cluster.test.name
  force_update_version = true
  node_group_name      = %[1]q
  node_role_arn        = aws_iam_role.node.arn
  subnet_ids           = aws_subnet.test[*].id
  version              = aws_eks_cluster.test.version

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigInstanceTypesMultiple(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ec2_instance_type_offerings" "available" {
  filter {
    name   = "instance-type"
    values = ["t3.medium", "t3.large", "t2.medium", "t2.large"]
  }
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  instance_types  = data.aws_ec2_instance_type_offerings.available.instance_types
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigInstanceTypesSingle(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ec2_instance_type_offering" "available" {
  filter {
    name   = "instance-type"
    values = ["t3.large", "t2.large"]
  }

  preferred_instance_types = ["t3.large", "t2.large"]
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  instance_types  = [data.aws_ec2_instance_type_offering.available.instance_type]
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLabels1(rName, labelKey1, labelValue1 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  labels = {
    %[2]q = %[3]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, labelKey1, labelValue1))
}

func testAccAWSEksNodeGroupConfigLabels2(rName, labelKey1, labelValue1, labelKey2, labelValue2 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  labels = {
    %[2]q = %[3]q
    %[4]q = %[5]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, labelKey1, labelValue1, labelKey2, labelValue2))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateId1(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test1" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-1"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_launch_template" "test2" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-2"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    id      = aws_launch_template.test1.id
    version = aws_launch_template.test1.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateId2(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test1" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-1"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_launch_template" "test2" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-2"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    id      = aws_launch_template.test2.id
    version = aws_launch_template.test2.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateName1(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test1" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-1"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_launch_template" "test2" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-2"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    name    = aws_launch_template.test1.name
    version = aws_launch_template.test1.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateName2(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test1" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-1"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_launch_template" "test2" {
  image_id      = data.aws_ssm_parameter.test.value
  instance_type = "t3.medium"
  name          = "%[1]s-2"
  user_data     = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    name    = aws_launch_template.test2.name
    version = aws_launch_template.test2.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateVersion1(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test" {
  image_id               = data.aws_ssm_parameter.test.value
  instance_type          = "t3.medium"
  name                   = %[1]q
  update_default_version = true
  user_data              = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    name    = aws_launch_template.test.name
    version = aws_launch_template.test.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigLaunchTemplateVersion2(rName string) string {
	return acctest.ConfigCompose(
		testAccAWSEksNodeGroupConfigBase(rName),
		fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/image_id"
}

resource "aws_launch_template" "test" {
  image_id               = data.aws_ssm_parameter.test.value
  instance_type          = "t3.large"
  name                   = %[1]q
  update_default_version = true
  user_data              = base64encode(templatefile("testdata/service/eks/node-group-launch-template-user-data.sh.tmpl", { cluster_name = aws_eks_cluster.test.name }))
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  launch_template {
    name    = aws_launch_template.test.name
    version = aws_launch_template.test.default_version
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigReleaseVersion(rName string, version string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBaseVersion(rName, version), fmt.Sprintf(`
data "aws_ssm_parameter" "test" {
  name = "/aws/service/eks/optimized-ami/${aws_eks_cluster.test.version}/amazon-linux-2/recommended/release_version"
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  release_version = data.aws_ssm_parameter.test.value
  subnet_ids      = aws_subnet.test[*].id
  version         = aws_eks_cluster.test.version

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigRemoteAccessEc2SshKey(rName, publicKey string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_key_pair" "test" {
  key_name   = %[1]q
  public_key = %[2]q
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  remote_access {
    ec2_ssh_key = aws_key_pair.test.key_name
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, publicKey))
}

func testAccAWSEksNodeGroupConfigRemoteAccessSourceSecurityGroupIds1(rName, publicKey string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_key_pair" "test" {
  key_name   = %[1]q
  public_key = %[2]q
}

resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  remote_access {
    ec2_ssh_key               = aws_key_pair.test.key_name
    source_security_group_ids = [aws_security_group.test.id]
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, publicKey))
}

func testAccAWSEksNodeGroupConfigScalingConfigSizes(rName string, desiredSize, maxSize, minSize int) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = %[2]d
    max_size     = %[3]d
    min_size     = %[4]d
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, desiredSize, maxSize, minSize))
}

func testAccAWSEksNodeGroupConfigTags1(rName, tagKey1, tagValue1 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  tags = {
    %[2]q = %[3]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, tagKey1, tagValue1))
}

func testAccAWSEksNodeGroupConfigTags2(rName, tagKey1, tagValue1, tagKey2, tagValue2 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  tags = {
    %[2]q = %[3]q
    %[4]q = %[5]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, tagKey1, tagValue1, tagKey2, tagValue2))
}

func testAccAWSEksNodeGroupConfigTaints1(rName, taintKey1, taintValue1, taintEffect1 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  taint {
    key    = %[2]q
    value  = %[3]q
    effect = %[4]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, taintKey1, taintValue1, taintEffect1))
}

func testAccAWSEksNodeGroupConfigTaints2(rName, taintKey1, taintValue1, taintEffect1, taintKey2, taintValue2, taintEffect2 string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  taint {
    key    = %[2]q
    value  = %[3]q
    effect = %[4]q
  }

  taint {
    key    = %[5]q
    value  = %[6]q
    effect = %[7]q
  }

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName, taintKey1, taintValue1, taintEffect1, taintKey2, taintValue2, taintEffect2))
}

func testAccAWSEksNodeGroupConfigUpdateConfig1(rName string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 3
    min_size     = 1
  }

  update_config {
    max_unavailable = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigUpdateConfig2(rName string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBase(rName), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id

  scaling_config {
    desired_size = 1
    max_size     = 3
    min_size     = 1
  }

  update_config {
    max_unavailable_percentage = 40
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}

func testAccAWSEksNodeGroupConfigVersion(rName, version string) string {
	return acctest.ConfigCompose(testAccAWSEksNodeGroupConfigBaseVersion(rName, version), fmt.Sprintf(`
resource "aws_eks_node_group" "test" {
  cluster_name    = aws_eks_cluster.test.name
  node_group_name = %[1]q
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.test[*].id
  version         = aws_eks_cluster.test.version

  scaling_config {
    desired_size = 1
    max_size     = 1
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.node-AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node-AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node-AmazonEC2ContainerRegistryReadOnly,
  ]
}
`, rName))
}
