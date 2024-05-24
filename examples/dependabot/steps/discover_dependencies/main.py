def discover_dependencies() -> list[dict]:
    return [
        dict(
            package='uvicorn',
            version='0.18.0',
            pkg_repo='https://pypi.org/project/uvicorn/',
            git_repo='https://github.com/encode/uvicorn'
        ),
        dict(
            package='next',
            version='0.13.0',
            pkg_repo='https://www.npmjs.com/package/next',
            git_repo='https://github.com/vercel/next.js'
        ),
    ]
