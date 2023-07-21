package redox

import "github.com/tidepool-org/clinic/redox/models"

var mrnIdentifierType = "MR"

func GetMRNFromOrder(order models.NewOrder) *string {
	for _, identifier := range order.Patient.Identifiers {
		if identifier.IDType == mrnIdentifierType {
			return &identifier.ID
		}
	}

	return nil
}
