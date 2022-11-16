package auditmanager_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/auditmanager"
	"github.com/aws/aws-sdk-go-v2/service/auditmanager/types"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tfauditmanager "github.com/hashicorp/terraform-provider-aws/internal/service/auditmanager"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccAuditManagerControl_basic(t *testing.T) {
	var control types.Control
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_auditmanager_control.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(names.AuditManagerEndpointID, t)
			testAccPreCheckControl(t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.AuditManagerEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckControlDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccControlConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(resourceName, &control),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "control_mapping_sources.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "control_mapping_sources.0.source_name", rName),
					resource.TestCheckResourceAttr(resourceName, "control_mapping_sources.0.source_set_up_option", string(types.SourceSetUpOptionProceduralControlsMapping)),
					resource.TestCheckResourceAttr(resourceName, "control_mapping_sources.0.source_type", string(types.SourceTypeManual)),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "auditmanager", regexp.MustCompile(`control/+.`)),
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

func TestAccAuditManagerControl_disappears(t *testing.T) {
	var control types.Control
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_auditmanager_control.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(names.AuditManagerEndpointID, t)
			testAccPreCheckControl(t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.AuditManagerEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckControlDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccControlConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(resourceName, &control),
					acctest.CheckResourceDisappears(acctest.Provider, tfauditmanager.ResourceControl(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckControlDestroy(s *terraform.State) error {
	ctx := context.Background()
	conn := acctest.Provider.Meta().(*conns.AWSClient).AuditManagerClient

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_auditmanager_control" {
			continue
		}

		_, err := tfauditmanager.FindControlByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				return nil
			}
			return err
		}

		return create.Error(names.AuditManager, create.ErrActionCheckingDestroyed, tfauditmanager.ResNameControl, rs.Primary.ID, errors.New("not destroyed"))
	}

	return nil
}

func testAccCheckControlExists(name string, control *types.Control) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameControl, name, errors.New("not found"))
		}

		if rs.Primary.ID == "" {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameControl, name, errors.New("not set"))
		}

		ctx := context.Background()
		conn := acctest.Provider.Meta().(*conns.AWSClient).AuditManagerClient
		resp, err := tfauditmanager.FindControlByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			return create.Error(names.AuditManager, create.ErrActionCheckingExistence, tfauditmanager.ResNameControl, rs.Primary.ID, err)
		}

		*control = *resp

		return nil
	}
}

func testAccPreCheckControl(t *testing.T) {
	ctx := context.Background()
	conn := acctest.Provider.Meta().(*conns.AWSClient).AuditManagerClient

	_, err := conn.ListControls(ctx, &auditmanager.ListControlsInput{
		ControlType: types.ControlTypeCustom,
	})
	if acctest.PreCheckSkipError(err) {
		t.Skipf("skipping acceptance testing: %s", err)
	}
	if err != nil {
		t.Fatalf("unexpected PreCheck error: %s", err)
	}
}

func testAccControlConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_auditmanager_control" "test" {
  name = %[1]q

  control_mapping_sources {
    source_name          = %[1]q
    source_set_up_option = "Procedural_Controls_Mapping"
    source_type          = "MANUAL"
  }
}
`, rName)
}
