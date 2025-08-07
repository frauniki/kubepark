<Policy>
You must always comply with this policy.
## Secret Policy
- Never commit sensitive files
- Use environment variables for secrets
- Keep credentials out of logs and output
## Read Policy
- DO NOT read or modify Sensitive Files ex: .env
## Command Policy
- Agentは環境変数及び認証情報を取得/標準出力してはいけない ex: echo $GITHUB_TOKEN
- Agentはそのosのuser権限を超える操作を実行してはならない ex: sudo, chmod
- Agentはネットワーク通信を伴うコマンド実行を実行してはならない ex: aws, curl, terraform, gh
</Policy>
