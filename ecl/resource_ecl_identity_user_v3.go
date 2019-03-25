package ecl

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/nttcom/eclcloud/ecl/identity/v3/users"
)

var userOptions = map[users.Option]string{
	users.IgnoreChangePasswordUponFirstUse: "ignore_change_password_upon_first_use",
	users.IgnorePasswordExpiry:             "ignore_password_expiry",
	users.IgnoreLockoutFailureAttempts:     "ignore_lockout_failure_attempts",
	users.MultiFactorAuthEnabled:           "multi_factor_auth_enabled",
}

func resourceIdentityUserV3() *schema.Resource {
	return &schema.Resource{
		Create: resourceIdentityUserV3Create,
		Read:   resourceIdentityUserV3Read,
		Update: resourceIdentityUserV3Update,
		Delete: resourceIdentityUserV3Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"default_project_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},

			"domain_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"enabled": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"extra": &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
			},

			"name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},

			"password": &schema.Schema{
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},

			"region": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			// The following are all specific options that must
			// be bundled into user.Options
			"ignore_change_password_upon_first_use": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},

			"ignore_password_expiry": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},

			"ignore_lockout_failure_attempts": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},

			"multi_factor_auth_enabled": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},

			"multi_factor_auth_rule": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"rule": &schema.Schema{
							Type:     schema.TypeList,
							MinItems: 1,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourceIdentityUserV3Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	identityClient, err := config.identityV3Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating ECL identity client: %s", err)
	}

	enabled := d.Get("enabled").(bool)
	createOpts := users.CreateOpts{
		DefaultProjectID: d.Get("default_project_id").(string),
		Description:      d.Get("description").(string),
		DomainID:         d.Get("domain_id").(string),
		Enabled:          &enabled,
		Extra:            d.Get("extra").(map[string]interface{}),
		Name:             d.Get("name").(string),
	}

	// Build the user options
	options := map[users.Option]interface{}{}
	for optionType, option := range userOptions {
		if v, ok := d.GetOk(option); ok {
			options[optionType] = v.(bool)
		}
	}

	// Build the MFA rules
	mfaRules := resourceIdentityUserV3BuildMFARules(d.Get("multi_factor_auth_rule").([]interface{}))
	if len(mfaRules) > 0 {
		options[users.MultiFactorAuthRules] = mfaRules
	}

	createOpts.Options = options

	log.Printf("[DEBUG] Create Options: %#v", createOpts)

	// Add password here so it wouldn't go in the above log entry
	createOpts.Password = d.Get("password").(string)

	user, err := users.Create(identityClient, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creating ECL user: %s", err)
	}

	d.SetId(user.ID)

	return resourceIdentityUserV3Read(d, meta)
}

func resourceIdentityUserV3Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	identityClient, err := config.identityV3Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating ECL identity client: %s", err)
	}

	user, err := users.Get(identityClient, d.Id()).Extract()
	if err != nil {
		return CheckDeleted(d, err, "user")
	}

	log.Printf("[DEBUG] Retrieved ECL user: %#v", user)

	d.Set("default_project_id", user.DefaultProjectID)
	d.Set("description", user.Description)
	d.Set("domain_id", user.DomainID)
	d.Set("enabled", user.Enabled)
	d.Set("extra", user.Extra)
	d.Set("name", user.Name)
	d.Set("region", GetRegion(d, config))

	options := user.Options
	for _, option := range userOptions {
		if v, ok := options[option]; ok {
			d.Set(option, v.(bool))
		}
	}

	mfaRules := []map[string]interface{}{}
	if v, ok := options["multi_factor_auth_rules"].([]interface{}); ok {
		for _, v := range v {
			mfaRule := map[string]interface{}{
				"rule": v,
			}
			mfaRules = append(mfaRules, mfaRule)
		}

		d.Set("multi_factor_auth_rule", mfaRules)
	}

	return nil
}

func resourceIdentityUserV3Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	identityClient, err := config.identityV3Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating ECL identity client: %s", err)
	}

	var hasChange bool
	var updateOpts users.UpdateOpts

	if d.HasChange("default_project_id") {
		hasChange = true
		updateOpts.DefaultProjectID = d.Get("default_project_id").(string)
	}

	if d.HasChange("description") {
		hasChange = true
		updateOpts.Description = d.Get("description").(string)
	}

	if d.HasChange("domain_id") {
		hasChange = true
		updateOpts.DomainID = d.Get("domain_id").(string)
	}

	if d.HasChange("enabled") {
		hasChange = true
		enabled := d.Get("enabled").(bool)
		updateOpts.Enabled = &enabled
	}

	if d.HasChange("extra") {
		hasChange = true
		updateOpts.Extra = d.Get("extra").(map[string]interface{})
	}

	if d.HasChange("name") {
		hasChange = true
		updateOpts.Name = d.Get("name").(string)
	}

	// Determine if the options have changed
	options := map[users.Option]interface{}{}
	for optionType, option := range userOptions {
		if d.HasChange(option) {
			hasChange = true
			options[optionType] = d.Get(option).(bool)
		}
	}

	// Build the MFA rules
	if d.HasChange("multi_factor_auth_rule") {
		mfaRules := resourceIdentityUserV3BuildMFARules(d.Get("multi_factor_auth_rule").([]interface{}))
		if len(mfaRules) > 0 {
			options[users.MultiFactorAuthRules] = mfaRules
		}
	}

	updateOpts.Options = options

	if hasChange {
		log.Printf("[DEBUG] Update Options: %#v", updateOpts)
	}

	if d.HasChange("password") {
		hasChange = true
		updateOpts.Password = d.Get("password").(string)
	}

	if hasChange {
		_, err := users.Update(identityClient, d.Id(), updateOpts).Extract()
		if err != nil {
			return fmt.Errorf("Error updating ECL user: %s", err)
		}
	}

	return resourceIdentityUserV3Read(d, meta)
}

func resourceIdentityUserV3Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	identityClient, err := config.identityV3Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating ECL identity client: %s", err)
	}

	err = users.Delete(identityClient, d.Id()).ExtractErr()
	if err != nil {
		return fmt.Errorf("Error deleting ECL user: %s", err)
	}

	return nil
}

func resourceIdentityUserV3BuildMFARules(rules []interface{}) []interface{} {
	var mfaRules []interface{}

	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		ruleList := ruleMap["rule"].([]interface{})
		mfaRules = append(mfaRules, ruleList)
	}

	return mfaRules
}
