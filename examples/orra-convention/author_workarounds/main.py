from libs.orra import orra
from src.author_workarounds import author_workarounds


@orra.after(activity='resolve')
def author_workarounds(state: dict) -> dict:
    print('orra - author_workarounds')
    author_workarounds()
    return state
