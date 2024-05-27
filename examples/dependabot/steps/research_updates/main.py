from gpt_researcher import GPTResearcher


async def research_update(dependency: dict) -> dict:
    if len(dependency) == 0:
        return {}

    query = f"What is the last released version of this ```{dependency['package']}``` library?"
    report_type = "research_report"
    sources = [dependency['pkg_repo'], dependency['git_repo']]

    researcher = GPTResearcher(query=query, report_type=report_type, source_urls=sources)
    await researcher.conduct_research()
    report = await researcher.write_report()

    return {
        "package": dependency['package'],
        "current_version": dependency['version'],
        "update": report
    }
