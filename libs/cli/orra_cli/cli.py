import typer
from rich import print
from typing import Annotated, Union

from . import __version__

app = typer.Typer()

def version_callback(value: bool):
    if value:
        print(f"CLI Version: {__version__}")
        raise typer.Exit()

@app.callback(no_args_is_help=True)
def callback(
        version: Annotated[Union[bool, None],
    typer.Option(
        default="--version",
        help="Show the version and exit.",
        callback=version_callback,
        is_eager=True
    ),
] = None) -> None:
    """
    Orra - orra is great!
    """

@app.command()
def run():
    print("Running Orra app.")

def main():
    app()
