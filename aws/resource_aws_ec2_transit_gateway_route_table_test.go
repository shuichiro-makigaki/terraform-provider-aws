package aws

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
)

func TestAccAWSEc2TransitGatewayRouteTable_basic(t *testing.T) {
	var transitGatewayRouteTable1 ec2.TransitGatewayRouteTable
	resourceName := "aws_ec2_transit_gateway_route_table.test"
	transitGatewayResourceName := "aws_ec2_transit_gateway.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEc2TransitGateway(t) },
		ErrorCheck:   acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEc2TransitGatewayRouteTableDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEc2TransitGatewayRouteTableConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable1),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "ec2", regexp.MustCompile(`transit-gateway-route-table/tgw-rtb-.+`)),
					resource.TestCheckResourceAttr(resourceName, "default_association_route_table", "false"),
					resource.TestCheckResourceAttr(resourceName, "default_propagation_route_table", "false"),
					resource.TestCheckResourceAttrPair(resourceName, "transit_gateway_id", transitGatewayResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
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

func TestAccAWSEc2TransitGatewayRouteTable_disappears(t *testing.T) {
	var transitGatewayRouteTable1 ec2.TransitGatewayRouteTable
	resourceName := "aws_ec2_transit_gateway_route_table.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEc2TransitGateway(t) },
		ErrorCheck:   acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEc2TransitGatewayRouteTableDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEc2TransitGatewayRouteTableConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable1),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceTransitGatewayRouteTable(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSEc2TransitGatewayRouteTable_disappears_TransitGateway(t *testing.T) {
	var transitGateway1 ec2.TransitGateway
	var transitGatewayRouteTable1 ec2.TransitGatewayRouteTable
	resourceName := "aws_ec2_transit_gateway_route_table.test"
	transitGatewayResourceName := "aws_ec2_transit_gateway.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEc2TransitGateway(t) },
		ErrorCheck:   acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEc2TransitGatewayRouteTableDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEc2TransitGatewayRouteTableConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayExists(transitGatewayResourceName, &transitGateway1),
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable1),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceTransitGateway(), transitGatewayResourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSEc2TransitGatewayRouteTable_Tags(t *testing.T) {
	var transitGatewayRouteTable1, transitGatewayRouteTable2, transitGatewayRouteTable3 ec2.TransitGatewayRouteTable
	resourceName := "aws_ec2_transit_gateway_route_table.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSEc2TransitGateway(t) },
		ErrorCheck:   acctest.ErrorCheck(t, ec2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSEc2TransitGatewayRouteTableDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEc2TransitGatewayRouteTableConfigTags1("key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable1),
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
				Config: testAccAWSEc2TransitGatewayRouteTableConfigTags2("key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable2),
					testAccCheckAWSEc2TransitGatewayRouteTableNotRecreated(&transitGatewayRouteTable1, &transitGatewayRouteTable2),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
			{
				Config: testAccAWSEc2TransitGatewayRouteTableConfigTags1("key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName, &transitGatewayRouteTable3),
					testAccCheckAWSEc2TransitGatewayRouteTableNotRecreated(&transitGatewayRouteTable2, &transitGatewayRouteTable3),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
		},
	})
}

func testAccCheckAWSEc2TransitGatewayRouteTableExists(resourceName string, transitGatewayRouteTable *ec2.TransitGatewayRouteTable) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No EC2 Transit Gateway Route Table ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Conn

		routeTable, err := ec2DescribeTransitGatewayRouteTable(conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		if routeTable == nil {
			return fmt.Errorf("EC2 Transit Gateway Route Table not found")
		}

		if aws.StringValue(routeTable.State) != ec2.TransitGatewayRouteTableStateAvailable {
			return fmt.Errorf("EC2 Transit Gateway Route Table (%s) exists in non-available (%s) state", rs.Primary.ID, aws.StringValue(routeTable.State))
		}

		*transitGatewayRouteTable = *routeTable

		return nil
	}
}

func testAccCheckAWSEc2TransitGatewayRouteTableDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Conn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_ec2_transit_gateway_route_table" {
			continue
		}

		routeTable, err := ec2DescribeTransitGatewayRouteTable(conn, rs.Primary.ID)

		if tfawserr.ErrMessageContains(err, "InvalidRouteTableID.NotFound", "") {
			continue
		}

		if err != nil {
			return err
		}

		if routeTable == nil {
			continue
		}

		if aws.StringValue(routeTable.State) != ec2.TransitGatewayRouteTableStateDeleted {
			return fmt.Errorf("EC2 Transit Gateway Route Table (%s) still exists in non-deleted (%s) state", rs.Primary.ID, aws.StringValue(routeTable.State))
		}
	}

	return nil
}

func testAccCheckAWSEc2TransitGatewayRouteTableNotRecreated(i, j *ec2.TransitGatewayRouteTable) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.StringValue(i.TransitGatewayRouteTableId) != aws.StringValue(j.TransitGatewayRouteTableId) {
			return errors.New("EC2 Transit Gateway Route Table was recreated")
		}

		return nil
	}
}

func testAccAWSEc2TransitGatewayRouteTableConfig() string {
	return `
resource "aws_ec2_transit_gateway" "test" {}

resource "aws_ec2_transit_gateway_route_table" "test" {
  transit_gateway_id = aws_ec2_transit_gateway.test.id
}
`
}

func testAccAWSEc2TransitGatewayRouteTableConfigTags1(tagKey1, tagValue1 string) string {
	return fmt.Sprintf(`
resource "aws_ec2_transit_gateway" "test" {}

resource "aws_ec2_transit_gateway_route_table" "test" {
  transit_gateway_id = aws_ec2_transit_gateway.test.id

  tags = {
    %q = %q
  }
}
`, tagKey1, tagValue1)
}

func testAccAWSEc2TransitGatewayRouteTableConfigTags2(tagKey1, tagValue1, tagKey2, tagValue2 string) string {
	return fmt.Sprintf(`
resource "aws_ec2_transit_gateway" "test" {}

resource "aws_ec2_transit_gateway_route_table" "test" {
  transit_gateway_id = aws_ec2_transit_gateway.test.id

  tags = {
    %q = %q
    %q = %q
  }
}
`, tagKey1, tagValue1, tagKey2, tagValue2)
}
