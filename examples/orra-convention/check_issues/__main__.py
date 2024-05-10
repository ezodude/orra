from src.check_issues import check_issues


@orra.root()
def check_issues(state: dict) -> dict:
    print('orra - check_issues')
    check_issues()
    return state
