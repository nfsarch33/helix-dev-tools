package patterns

// ShellDenyPatterns are regex patterns that MUST be blocked.
// Ported from guard-shell.sh DENY_PATTERNS array.
var ShellDenyPatterns = []string{
	// Filesystem destruction
	`rm -rf /`,
	`rm -rf /[a-z]`,
	`rm -rf ~`,
	`rm -rf \$HOME`,
	`rm -rf /home`,
	`rm -rf /usr`,
	`rm -rf /etc`,
	`rm -rf /var`,
	`rm -rf /boot`,
	`rm -rf /System`,
	`rm -rf /Applications`,
	`rm -rf /Library`,
	`rm -Rf /`,
	`rm -fr /`,

	// Disk/partition destruction
	`mkfs[.]`,
	`dd if=/dev/`,
	`wipefs`,
	`fdisk`,
	`parted`,
	`diskutil eraseDisk`,
	`diskutil partitionDisk`,

	// Remote code execution
	`curl.*[|].*bash`,
	`curl.*[|].*sh[^a-z]`,
	`curl.*[|].*python`,
	`curl.*[|].*perl`,
	`curl.*[|].*ruby`,
	`wget.*[|].*bash`,
	`wget.*[|].*sh[^a-z]`,
	`wget.*[|].*python`,
	`eval.*curl`,
	`eval.*wget`,
	`bash -c.*curl`,
	`bash.*<.*curl`,

	// Privilege escalation
	`chmod -R 777 /`,
	`chmod 777 /`,
	`chown -R.*/`,
	`chown.*root`,

	// Git destruction on protected branches
	`git push.*--force.*main$`,
	`git push.*--force.*master$`,
	`git push.*-f .*main$`,
	`git push.*-f .*master$`,
	`git reset --hard.*origin`,
	`git clean -fdx /`,

	// Database destruction
	`DROP DATABASE`,
	`DROP TABLE`,
	`DROP SCHEMA`,
	`TRUNCATE `,
	`DELETE FROM.*WHERE 1`,
	`DELETE FROM.*WITHOUT`,

	// Fork bomb / resource exhaustion
	`[:][(][)][{]`,
	`while true.*do.*done`,
	`yes [|] `,

	// Credential exfiltration
	`cat.*[.]env.*[|].*curl`,
	`cat.*id_rsa.*[|]`,
	`cat.*[.]ssh.*[|].*nc`,
	`base64.*[.]env`,
	`base64.*id_rsa`,
	`base64.*credentials`,

	// Container escape
	`docker run.*--privileged`,
	`docker run.*-v /:/host`,
	`nsenter.*--target 1`,

	// Reverse shells
	`bash -i.*[>][&].*dev/tcp`,
	`nc -e.*bin/bash`,
	`nc -e.*bin/sh`,
	`python.*socket.*connect`,
	`ncat.*-e`,
	`socat.*exec`,

	// System service manipulation
	`systemctl stop`,
	`systemctl disable`,
	`launchctl unload`,
	`killall -9`,
	`kill -9 1$`,
	`shutdown`,
	`reboot`,
	`halt`,
	`init 0`,
	`init 6`,

	// Crontab manipulation
	`crontab -r`,
	`crontab.*[|].*curl`,

	// SSH key theft
	`ssh-keygen.*-f /tmp`,
	`cat.*authorized_keys.*[|]`,
	`echo.*>>.*authorized_keys`,

	// Environment pollution
	`export.*PATH=/`,
	`export.*LD_PRELOAD`,
	`export.*DYLD_INSERT_LIBRARIES`,

	// Wildcard/find destruction
	`rm -rf /\*`,
	`rm -rf /\.`,
	`find / -delete`,
	`find / .*-exec.*rm`,
	`find / .*xargs.*rm`,

	// Device writes
	`> /dev/sd`,
	`> /dev/nvme`,
	`> /dev/disk`,
	`mv / `,
	`mv /\* `,

	// Permission lockout
	`chmod -R 000 /`,
	`chmod 000 /`,

	// History/log destruction
	`history -c.*&&.*rm`,
	`shred.*auth.*log`,
	`shred.*/var/log`,

	// Kernel/firmware
	`modprobe.*-r`,
	`rmmod `,
	`insmod `,
}

// ShellWarnPatterns require user confirmation.
// Ported from guard-shell.sh WARN_PATTERNS array.
var ShellWarnPatterns = []string{
	`git push.*--force`,
	`^sudo `,
	`npm install -g`,
	`^pip install`,
	`docker run.*--rm`,
	`brew uninstall`,
	`apt remove`,
	`apt purge`,
	`rm -rf `,
	`git stash drop`,
	`git branch -D`,
	`docker system prune`,
	`docker volume rm`,
}

// PrePushChecks maps repo directory names to the formatter command that
// MUST be run before pushing. The guard-shell hook can reference this
// to emit a soft warning when a git push targets one of these repos
// without a recent formatter run in the session.
var PrePushChecks = map[string]string{
	"zendesk_console":         "prettier --write",
	"contact-center-frontend": "prettier --write",
	"contact-center":          "gofmt -w",
}

// MCPDenyTools are MCP tool names that MUST be blocked.
// Ported from guard-mcp.sh DENY_TOOLS array.
var MCPDenyTools = []string{
	"delete_repository",
	"delete_branch",
	"delete_issue",
	"delete_pull_request",
	"drop_database",
	"drop_collection",
	"delete_project",
	"destroy_resource",
	"force_merge",
}

// MCPWarnTools are MCP tool names requiring user confirmation.
// Ported from guard-mcp.sh WARN_PATTERNS array.
var MCPWarnTools = []string{
	"create_pull_request",
	"merge_pull_request",
	"create_issue",
	"push_files",
	"create_or_update_file",
	"update_issue",
	"close_issue",
	"create_branch",
	"jira_create_issue",
	"jira_update_issue",
	"jira_transition_issue",
}
