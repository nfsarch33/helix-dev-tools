package patterns

// BlockedFilenames are exact filenames that must not be read.
// Ported from sanitize-read.sh BLOCKED_FILES array.
var BlockedFilenames = []string{
	".env",
	".env.local",
	".env.development",
	".env.production",
	".env.staging",
	".env.test",
	"credentials.json",
	"service-account.json",
	"service-account-key.json",
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
	".npmrc",
	".pypirc",
	"token.json",
	"tokens.json",
	"secrets.yaml",
	"secrets.yml",
	"secrets.json",
	"vault.json",
	".netrc",
	".docker/config.json",
	"kubeconfig",
	".kube/config",
	"htpasswd",
	".pgpass",
	"private.pem",
	"private.key",
	"key.pem",
	"server.key",
}

// BlockedDirs are path substrings that indicate secrets directories.
// Ported from sanitize-read.sh BLOCKED_DIRS array.
var BlockedDirs = []string{
	"/.ssh/",
	"/.gnupg/",
	"/.aws/",
	"/.gcloud/",
	"/.azure/",
	"/.kube/",
	"/.docker/",
	"/secrets/",
	"/private/",
	"/.cursor/mcp.json",
	"/.config/gcloud/",
	"/.config/op/",
	"/.password-store/",
	"/.local/share/keyrings/",
}

// BlockedExtensions are file extensions for key/certificate files.
var BlockedExtensions = []string{
	".pem",
	".key",
	".p12",
	".pfx",
	".jks",
	".keystore",
}
