package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type petitionSearchField struct {
	Field       string `json:"field"`
	Operations  string `json:"operations"`
	Description string `json:"description"`
}

var petitionSearchFieldCatalog = []petitionSearchField{
	{"petitionDecisionRecordIdentifier", "fields,filter,sort", "Petition decision record identifier"},
	{"applicationNumberText", "fields,filter,sort", "Application number"},
	{"patentNumber", "fields,filter,sort", "Patent number"},
	{"businessEntityStatusCategory", "fields,filter,facet,sort", "Business entity status"},
	{"customerNumber", "fields,filter,sort", "Customer number"},
	{"decisionDate", "fields,filter,range,sort", "Decision date"},
	{"decisionPetitionTypeCode", "fields,filter,facet,sort", "Petition type code"},
	{"decisionPetitionTypeCodeDescriptionText", "fields,filter,facet,sort", "Petition type description"},
	{"decisionTypeCode", "fields,filter,sort", "Decision type code"},
	{"decisionTypeCodeDescriptionText", "fields,filter,facet,sort", "Decision outcome description"},
	{"finalDecidingOfficeName", "fields,filter,facet,sort", "Deciding USPTO office"},
	{"firstApplicantName", "fields,filter,sort", "First applicant"},
	{"firstInventorToFileIndicator", "fields,filter,facet", "First inventor to file indicator"},
	{"groupArtUnitNumber", "fields,filter,sort", "Group art unit"},
	{"technologyCenter", "fields,filter,facet,sort", "Technology center"},
	{"inventionTitle", "fields,filter,sort", "Invention title"},
	{"inventorBag", "fields,filter", "Inventor names"},
	{"courtActionIndicator", "fields,filter,facet", "Court action indicator"},
	{"actionTakenByCourtName", "fields,filter,facet", "Court action name"},
	{"petitionMailDate", "fields,filter,range,sort", "Petition mail date"},
	{"prosecutionStatusCode", "fields,filter,sort", "Prosecution status code"},
	{"prosecutionStatusCodeDescriptionText", "fields,filter,facet,sort", "Prosecution status description"},
	{"petitionIssueConsideredTextBag", "fields,filter,facet", "Issues considered"},
	{"ruleBag", "fields,filter,facet", "Rules cited"},
	{"statuteBag", "fields,filter,facet", "Statutes cited"},
	{"lastIngestionDateTime", "fields,filter,range,sort", "Last ingestion timestamp"},
}

var petitionFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List Petition Decision search fields",
	Long:  "Lists the Swagger-documented Petition Decision fields and their supported CLI operations.",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputResult(cmd, petitionSearchFieldCatalog, nil)
		return nil
	},
}

func init() {
	petitionCmd.AddCommand(petitionFieldsCmd)
}

func validatePetitionSearchFields() error {
	allowed := make(map[string]string, len(petitionSearchFieldCatalog))
	for _, field := range petitionSearchFieldCatalog {
		allowed[field.Field] = field.Operations
	}
	if err := validatePetitionFieldList("--fields", splitCommaValues(petitionSearchFlags.fields), allowed, "fields"); err != nil {
		return err
	}
	if err := validatePetitionFieldList("--facets", splitCommaValues(petitionSearchFlags.facets), allowed, "facet"); err != nil {
		return err
	}
	filters, err := parseStructuredFilters(petitionSearchFlags.filters)
	if err != nil {
		return err
	}
	for _, filter := range filters {
		if err := validatePetitionFieldList("--filter", []string{filter.Name}, allowed, "filter"); err != nil {
			return err
		}
	}
	ranges, err := parseRangeFilters(petitionSearchFlags.ranges)
	if err != nil {
		return err
	}
	for _, filter := range ranges {
		if err := validatePetitionFieldList("--range", []string{filter.Field}, allowed, "range"); err != nil {
			return err
		}
	}
	if sort := strings.TrimSpace(petitionSearchFlags.sort); sort != "" {
		field := strings.TrimSpace(strings.SplitN(sort, ":", 2)[0])
		if err := validatePetitionFieldList("--sort", []string{field}, allowed, "sort"); err != nil {
			return err
		}
	}
	return nil
}

func validatePetitionFieldList(flag string, fields []string, allowed map[string]string, operation string) error {
	for _, field := range fields {
		operations, ok := allowed[field]
		if ok && strings.Contains(operations, operation) {
			continue
		}
		if !ok {
			return fmt.Errorf("unsupported %s field %q; run `uspto petition fields` for supported fields", flag, field)
		}
		return fmt.Errorf("field %q cannot be used with %s; run `uspto petition fields` for supported operations", field, flag)
	}
	return nil
}
