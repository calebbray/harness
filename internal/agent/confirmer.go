package agent

import "encoding/json"

type Confirmer func(toolName string, input json.RawMessage) bool
