package sfn

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceStateMachine() *schema.Resource {
	return &schema.Resource{
		Create: resourceStateMachineCreate,
		Read:   resourceStateMachineRead,
		Update: resourceStateMachineUpdate,
		Delete: resourceStateMachineDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"creation_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"definition": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(0, 1024*1024), // 1048576
			},
			"logging_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"include_execution_data": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"level": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(sfn.LogLevel_Values(), false),
						},
						"log_destination": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validStateMachineName,
			},
			"role_arn": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: verify.ValidARN,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
			"tracing_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
				DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      sfn.StateMachineTypeStandard,
				ValidateFunc: validation.StringInSlice(sfn.StateMachineType_Values(), false),
			},
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceStateMachineCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).SFNConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	name := d.Get("name").(string)
	input := &sfn.CreateStateMachineInput{
		Definition: aws.String(d.Get("definition").(string)),
		Name:       aws.String(name),
		RoleArn:    aws.String(d.Get("role_arn").(string)),
		Tags:       Tags(tags.IgnoreAWS()),
		Type:       aws.String(d.Get("type").(string)),
	}

	if v, ok := d.GetOk("logging_configuration"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
		input.LoggingConfiguration = expandLoggingConfiguration(v.([]interface{})[0].(map[string]interface{}))
	}

	if v, ok := d.GetOk("tracing_configuration"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
		input.TracingConfiguration = expandTracingConfiguration(v.([]interface{})[0].(map[string]interface{}))
	}

	// This is done to deal with IAM eventual consistency.
	// Note: the instance may be in a deleting mode, hence the retry
	// when creating the step function. This can happen when we are
	// updating the resource (since there is no update API call).
	outputRaw, err := tfresource.RetryWhenAWSErrCodeEquals(stateMachineCreatedTimeout, func() (interface{}, error) {
		return conn.CreateStateMachine(input)
	}, sfn.ErrCodeStateMachineDeleting, "AccessDeniedException")

	if err != nil {
		return fmt.Errorf("creating Step Functions State Machine (%s): %w", name, err)
	}

	d.SetId(aws.StringValue(outputRaw.(*sfn.CreateStateMachineOutput).StateMachineArn))

	return resourceStateMachineRead(d, meta)
}

func resourceStateMachineRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).SFNConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	output, err := FindStateMachineByARN(conn, d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Step Functions State Machine (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading Step Functions State Machine (%s): %w", d.Id(), err)
	}

	d.Set("arn", output.StateMachineArn)
	if output.CreationDate != nil {
		d.Set("creation_date", aws.TimeValue(output.CreationDate).Format(time.RFC3339))
	} else {
		d.Set("creation_date", nil)
	}
	d.Set("definition", output.Definition)
	if output.LoggingConfiguration != nil {
		if err := d.Set("logging_configuration", []interface{}{flattenLoggingConfiguration(output.LoggingConfiguration)}); err != nil {
			return fmt.Errorf("error setting logging_configuration: %w", err)
		}
	} else {
		d.Set("logging_configuration", nil)
	}
	d.Set("name", output.Name)
	d.Set("role_arn", output.RoleArn)
	d.Set("status", output.Status)
	if output.TracingConfiguration != nil {
		if err := d.Set("tracing_configuration", []interface{}{flattenTracingConfiguration(output.TracingConfiguration)}); err != nil {
			return fmt.Errorf("error setting tracing_configuration: %w", err)
		}
	} else {
		d.Set("tracing_configuration", nil)
	}
	d.Set("type", output.Type)

	tags, err := ListTags(conn, d.Id())

	if tfawserr.ErrCodeEquals(err, "UnknownOperationException") {
		return nil
	}

	if err != nil {
		return fmt.Errorf("listing tags for Step Functions State Machine (%s): %w", d.Id(), err)
	}

	tags = tags.IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("setting tags_all: %w", err)
	}

	return nil
}

func resourceStateMachineUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).SFNConn

	if d.HasChangesExcept("tags", "tags_all") {
		// "You must include at least one of definition or roleArn or you will receive a MissingRequiredParameter error"
		input := &sfn.UpdateStateMachineInput{
			Definition:      aws.String(d.Get("definition").(string)),
			RoleArn:         aws.String(d.Get("role_arn").(string)),
			StateMachineArn: aws.String(d.Id()),
		}

		if d.HasChange("logging_configuration") {
			if v, ok := d.GetOk("logging_configuration"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
				input.LoggingConfiguration = expandLoggingConfiguration(v.([]interface{})[0].(map[string]interface{}))
			}
		}

		if d.HasChange("tracing_configuration") {
			if v, ok := d.GetOk("tracing_configuration"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
				input.TracingConfiguration = expandTracingConfiguration(v.([]interface{})[0].(map[string]interface{}))
			}
		}

		_, err := conn.UpdateStateMachine(input)

		if err != nil {
			return fmt.Errorf("updating Step Functions State Machine (%s): %w", d.Id(), err)
		}

		// Handle eventual consistency after update.
		err = resource.Retry(stateMachineUpdatedTimeout, func() *resource.RetryError {
			output, err := FindStateMachineByARN(conn, d.Id())

			if err != nil {
				return resource.NonRetryableError(err)
			}

			if d.HasChange("definition") && !verify.JSONBytesEqual([]byte(aws.StringValue(output.Definition)), []byte(d.Get("definition").(string))) ||
				d.HasChange("role_arn") && aws.StringValue(output.RoleArn) != d.Get("role_arn").(string) ||
				d.HasChange("tracing_configuration.0.enabled") && output.TracingConfiguration != nil && aws.BoolValue(output.TracingConfiguration.Enabled) != d.Get("tracing_configuration.0.enabled").(bool) ||
				d.HasChange("logging_configuration.0.include_execution_data") && output.LoggingConfiguration != nil && aws.BoolValue(output.LoggingConfiguration.IncludeExecutionData) != d.Get("logging_configuration.0.include_execution_data").(bool) ||
				d.HasChange("logging_configuration.0.level") && output.LoggingConfiguration != nil && aws.StringValue(output.LoggingConfiguration.Level) != d.Get("logging_configuration.0.level").(string) {
				return resource.RetryableError(fmt.Errorf("Step Functions State Machine (%s) eventual consistency", d.Id()))
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("waiting for Step Functions State Machine (%s) update: %w", d.Id(), err)
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return fmt.Errorf("updating Step Functions State Machine (%s) tags: %w", d.Id(), err)
		}
	}

	return resourceStateMachineRead(d, meta)
}

func resourceStateMachineDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).SFNConn

	log.Printf("[DEBUG] Deleting Step Functions State Machine: %s", d.Id())
	_, err := conn.DeleteStateMachine(&sfn.DeleteStateMachineInput{
		StateMachineArn: aws.String(d.Id()),
	})

	if err != nil {
		return fmt.Errorf("deleting Step Functions State Machine (%s): %w", d.Id(), err)
	}

	if _, err := waitStateMachineDeleted(conn, d.Id()); err != nil {
		return fmt.Errorf("waiting for Step Functions State Machine (%s) delete: %w", d.Id(), err)
	}

	return nil
}

func FindStateMachineByARN(conn *sfn.SFN, arn string) (*sfn.DescribeStateMachineOutput, error) {
	input := &sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(arn),
	}

	output, err := conn.DescribeStateMachine(input)

	if tfawserr.ErrCodeEquals(err, sfn.ErrCodeStateMachineDoesNotExist) {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output, nil
}

func statusStateMachine(conn *sfn.SFN, stateMachineArn string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindStateMachineByARN(conn, stateMachineArn)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.StringValue(output.Status), nil
	}
}

const (
	stateMachineCreatedTimeout = 5 * time.Minute
	stateMachineDeletedTimeout = 5 * time.Minute
	stateMachineUpdatedTimeout = 1 * time.Minute
)

func waitStateMachineDeleted(conn *sfn.SFN, stateMachineArn string) (*sfn.DescribeStateMachineOutput, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{sfn.StateMachineStatusActive, sfn.StateMachineStatusDeleting},
		Target:  []string{},
		Refresh: statusStateMachine(conn, stateMachineArn),
		Timeout: stateMachineDeletedTimeout,
	}

	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*sfn.DescribeStateMachineOutput); ok {
		return output, err
	}

	return nil, err
}

func expandLoggingConfiguration(tfMap map[string]interface{}) *sfn.LoggingConfiguration {
	if tfMap == nil {
		return nil
	}

	apiObject := &sfn.LoggingConfiguration{}

	if v, ok := tfMap["include_execution_data"].(bool); ok {
		apiObject.IncludeExecutionData = aws.Bool(v)
	}

	if v, ok := tfMap["level"].(string); ok && v != "" {
		apiObject.Level = aws.String(v)
	}

	if v, ok := tfMap["log_destination"].(string); ok && v != "" {
		apiObject.Destinations = []*sfn.LogDestination{{
			CloudWatchLogsLogGroup: &sfn.CloudWatchLogsLogGroup{
				LogGroupArn: aws.String(v),
			},
		}}
	}

	return apiObject
}

func flattenLoggingConfiguration(apiObject *sfn.LoggingConfiguration) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.IncludeExecutionData; v != nil {
		tfMap["include_execution_data"] = aws.BoolValue(v)
	}

	if v := apiObject.Level; v != nil {
		tfMap["level"] = aws.StringValue(v)
	}

	if v := apiObject.Destinations; len(v) > 0 {
		tfMap["log_destination"] = aws.StringValue(v[0].CloudWatchLogsLogGroup.LogGroupArn)
	}

	return tfMap
}

func expandTracingConfiguration(tfMap map[string]interface{}) *sfn.TracingConfiguration {
	if tfMap == nil {
		return nil
	}

	apiObject := &sfn.TracingConfiguration{}

	if v, ok := tfMap["enabled"].(bool); ok {
		apiObject.Enabled = aws.Bool(v)
	}

	return apiObject
}

func flattenTracingConfiguration(apiObject *sfn.TracingConfiguration) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Enabled; v != nil {
		tfMap["enabled"] = aws.BoolValue(v)
	}

	return tfMap
}
