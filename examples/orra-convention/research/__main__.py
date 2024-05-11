from src.research import research


@orra.after(activity='root')
def research(state:dict) -> dict:
    print('orra - research')
    research()
    return state
