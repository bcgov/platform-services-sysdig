package helpers

import "strings"

// TeamFacts holds computed values for team namespaces and names.
type TeamFacts struct {
	NSPrefix                string
	Namespaces              []string
	ProdNamespace           string
	ContainerTeamName       string
	ContainerSecureTeamName string
	HostTeamName            string
	ContainerTeamExists     bool
	HostTeamExists          bool
}

// SetTeamFacts computes the necessary facts given the current namespace.
// It splits the namespace on "-tools" and computes values based on the prefix.
func SetTeamFacts(namespace string) TeamFacts {
	// Split the namespace by "-tools" and lowercase the prefix
	nsPrefixParts := strings.Split(namespace, "-tools")
	nsPrefix := strings.ToLower(nsPrefixParts[0])

	// Define the list of namespaces and related names
	namespaces := []string{
		nsPrefix + "-tools",
		nsPrefix + "-dev",
		nsPrefix + "-test",
		nsPrefix + "-prod",
	}

	return TeamFacts{
		NSPrefix:                nsPrefix,
		Namespaces:              namespaces,
		ProdNamespace:           nsPrefix + "-prod",
		ContainerTeamName:       nsPrefix + "-team",
		ContainerSecureTeamName: nsPrefix + "-team-secure",
		HostTeamName:            nsPrefix + "-team-persistent-storage",
		ContainerTeamExists:     false,
		HostTeamExists:          false,
	}
}
