from libs.orra import orra


@orra.after(activity='research')
def resolve(state:dict) -> dict:
    print('orra - resolve')
    return state
