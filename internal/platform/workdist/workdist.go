package workdist

type Ticket struct {
	ID         string
	Capability string
}

type Agent struct {
	ID           string
	Capabilities []string
}

func DistributeRoundRobin(tickets []Ticket, agents []Agent) map[string][]Ticket {
	result := make(map[string][]Ticket)
	if len(agents) == 0 {
		return result
	}
	for _, a := range agents {
		result[a.ID] = nil
	}
	for i, t := range tickets {
		agentIdx := i % len(agents)
		aid := agents[agentIdx].ID
		result[aid] = append(result[aid], t)
	}
	return result
}

func DistributeByCapability(tickets []Ticket, agents []Agent) map[string][]Ticket {
	result := make(map[string][]Ticket)
	for _, a := range agents {
		result[a.ID] = nil
	}

	capIndex := make(map[string][]string)
	for _, a := range agents {
		for _, c := range a.Capabilities {
			capIndex[c] = append(capIndex[c], a.ID)
		}
	}

	counters := make(map[string]int)
	for _, t := range tickets {
		candidates := capIndex[t.Capability]
		if len(candidates) == 0 {
			if len(agents) > 0 {
				aid := agents[0].ID
				result[aid] = append(result[aid], t)
			}
			continue
		}
		idx := counters[t.Capability] % len(candidates)
		aid := candidates[idx]
		result[aid] = append(result[aid], t)
		counters[t.Capability]++
	}
	return result
}
