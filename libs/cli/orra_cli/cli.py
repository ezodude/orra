import typer
from . import __version__

app = typer.Typer()


def version_callback(value: bool):
    if value:
        typer.echo(f"CLI Version: {__version__}")
        raise typer.Exit()


@app.callback()
def callback(
        version: bool = typer.Option(None, "--version",
                                     callback=version_callback,
                                     is_eager=True,
                                     help="Show the version and exit.")
):
    if version is None:
        typer.echo("Welcome to Orra CLI. Use --help to see commands")


def main():
    app()
