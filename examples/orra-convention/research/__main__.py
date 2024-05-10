from src.research import research


@orra.after('root')
def research(state:dict) -> dict:
    print('orra - research')
    research()
    return state
