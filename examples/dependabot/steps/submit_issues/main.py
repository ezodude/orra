from textwrap import dedent


def submit_issues(drafted_issues) -> []:
    curl_commands = []
    for issue in drafted_issues:
        issue_title = issue['title']
        issue_body = issue['description']
        issue_label = issue['label']
        curl_command = dedent(r"""\
curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/OWNER/REPO/issues \
  -d '{{"title":"{issue_title}","body":"{issue_body}","labels":["{issue_label}"]}}'
""").format(issue_title=issue_title, issue_body=issue_body, issue_label=issue_label)
        curl_commands.append(curl_command)

    print("\n\n".join(curl_commands))
    return curl_commands
