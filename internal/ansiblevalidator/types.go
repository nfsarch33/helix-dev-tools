package ansiblevalidator

// InventoryGroup represents a named group in an Ansible inventory.
type InventoryGroup struct {
	Name  string
	Hosts map[string]map[string]interface{}
	Vars  map[string]interface{}
}

// Inventory represents a parsed Ansible inventory YAML file.
type Inventory struct {
	Groups map[string]InventoryGroup
}

// PlaybookPlay represents a single play in an Ansible playbook.
type PlaybookPlay struct {
	Name        string
	Hosts       string
	GatherFacts *bool
	Vars        map[string]interface{}
	Tasks       []map[string]interface{}
}

// ValidationResult holds the outcome of a validation run.
type ValidationResult struct {
	Valid  bool
	Errors []string
}
