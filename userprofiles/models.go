package userprofiles

type KeycloakUserAttrEvent struct {
	Op     string       `json:"op"`
	Before *UserAttrDAO `json:"before"`
	After  *UserAttrDAO `json:"after"`
}

type UserAttrDAO struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	UserId string `json:"user_id"`
	Value  string `json:"value"`
}

func (e KeycloakUserAttrEvent) ShouldApplyUpdates() bool {
	return e.Op == "u" && e.After != nil && e.After.UserId != "" && e.After.Name == "full_name" && e.Before != nil && e.Before.Value != e.After.Value
}
