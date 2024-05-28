import json
from textwrap import dedent

from crewai import Agent, Task, Crew
from langchain_community.tools.tavily_search import TavilySearchResults
from langchain_openai import ChatOpenAI
from pydantic import BaseModel


class DraftIssue(BaseModel):
    package: str
    title: str
    description: str
    label: str


class DraftIssues(BaseModel):
    draft_issues: list[DraftIssue]


def create_crew(llm, package_updates):
    software_updates_reviewer = Agent(
        role="Software Updates Reviewer",
        goal="Analyze required package updates.",
        backstory="""You are a software updates reviewer responsible for analyzing software dependency version updates. 
        You have experience working with software libraries and updating dependencies to ensure that the software 
        is up-to-date and secure.""",
        verbose=False,
        memory=False,
        allow_delegation=False,
        cache=False,
        llm=llm
    )

    senior_developer_agent = Agent(
        role="Senior Software Developer",
        goal="Draft GitHub issues that clearly describe the required dependency version update and how.",
        backstory="""You are a senior software developer with experience in maintaining open-source projects. You work 
        with a team of developers to ensure that the project's dependencies are up-to-date and secure. You have 
        experience drafting GitHub issues that clearly describe what dependencies need to be updated and how.""",
        tools=[TavilySearchResults()],
        verbose=False,
        memory=False,
        allow_delegation=False,
        cache=False,
        llm=llm
    )

    describe_updates = Task(
        description=dedent(f"""\
For every research result, analyze the software library package and its research to determine the required
dependency update version, the required command to update the dependency, and the reasons why the update
is necessary.

PACKAGE VERSION UPDATES
-----------------------
{package_updates}

Your final answer MUST be the relevant package, it's current version, the new update version,
a code snippet to perform the update and the reason why the update is necessary, USE BULLET POINTS.
            """),
        agent=software_updates_reviewer,
        expected_output="A bullet pointed list that includes the package name, the current version, the "
                        "new update version, a code snippet to perform the update, and the reason why the update "
                        "is necessary.",
    )

    draft_issues = Task(
        description=dedent(f"""\
Draft a GitHub issue for the software library package that clearly describes the required dependency update 
version, the required command to update the dependency, and the reasons why the update is necessary.
        
The created GitHub issue should be clear, concise, and informative. It should provide all the necessary 
information in separate sections. The sections are:
- A well-defined title that summarizes the issue in a maximum of 10 words.
- A detailed description explains the necessary details for each dependency update.
        
You MUST create all drafts before sending your final answer.
        
Your final answer MUST be a JSON list that ONLY includes an entry for every reviewed package. Each entry will include 
the package name, the drafted issue details and a label to categorize the issue.
            """),
        agent=senior_developer_agent,
        expected_output="A JSON list that ONLY includes an entry for every reviewed package. Each entry will include"
                        "the package name, the drafted issue details and a label to categorize the issue.",
        output_json=DraftIssues,
    )

    crew = Crew(
        agents=[software_updates_reviewer, senior_developer_agent],
        tasks=[describe_updates, draft_issues],
        full_output=True,
        verbose=0,
    )

    return crew


def run_draft_issues(package_updates):
    if len(package_updates) == 0:
        return {}

    llm = ChatOpenAI(
        model="gpt-4o",
        temperature=0.9)

    crew = create_crew(llm, _format_package_updates(package_updates))
    full_output = crew.kickoff()
    # noinspection PyTypeChecker
    raw = full_output['final_output']
    result = json.loads(raw)['draft_issues']
    return result


def _format_package_updates(src):
    out = []
    for entry in src:
        details = [
            f"- Software Package: {entry['package']}",
            f"- Current Package Version: {entry['current_version']}",
            f"- Version Update Research (as Markdown):\n\n{entry['update']}",
            f"------------------------------------------------------------------",  # Separator
        ]
        out.append("\n\n".join(details))
    result = "\n\n".join(out)
    return result
