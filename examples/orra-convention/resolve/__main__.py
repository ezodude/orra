
@orra.after(activity='research')
def resolve(state:dict) -> dict:
    print('orra - resolve')
    return state
