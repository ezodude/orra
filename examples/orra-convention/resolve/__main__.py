
@orra.after('research')
def resolve(state:dict) -> dict:
    print('orra - resolve')
    return state
