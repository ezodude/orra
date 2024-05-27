import asyncio
import json
from typing import Any
from typing import Optional, List, Dict

from dotenv import load_dotenv

load_dotenv()

from orra import Orra

import steps

app = Orra(
    schema={
        "dependencies": Optional[List[Dict]],
        "researched": Optional[List[Dict]],
        "drafted": Optional[List[Dict]],
    }
)

data = """
{
	"dependencies": [
		{
			"package": "uvicorn",
			"version": "0.18.0",
			"pkg_repo": "https://pypi.org/project/uvicorn/",
			"git_repo": "https://github.com/encode/uvicorn"
		},
		{
			"package": "next",
			"version": "0.13.0",
			"pkg_repo": "https://www.npmjs.com/package/next",
			"git_repo": "https://github.com/vercel/next.js"
		}
	],
	"researched": [
		{
			"package": "uvicorn",
			"current_version": "0.18.0",
			"update": "# Detailed Report on the Latest Released Version of the Uvicorn Library\\n\\n## Latest Released Version\\n\\nThe latest released version of the Uvicorn library is **0.29.0**. This version was released on March 20, 2024. The release of version 0.29.0 marks a significant update in the Uvicorn library, reflecting ongoing improvements and optimizations to enhance its performance and capabilities.\\n\\n## Key Features and Improvements in Version 0.29.0\\n\\n### Performance Enhancements\\n\\nOne of the primary focuses of Uvicorn 0.29.0 is performance optimization. The ASGI server is known for its speed, and this version continues to build on that reputation. The update includes several under-the-hood improvements that reduce latency and increase throughput, making it even more suitable for high-traffic applications.\\n\\n### Compatibility and Support\\n\\nUvicorn 0.29.0 maintains its support for HTTP/1.1 and WebSockets, ensuring compatibility with a wide range of web applications. Additionally, it continues to support Python versions 3.8 and above, aligning with the latest Python developments and ensuring that developers can leverage the newest features of the language.\\n\\n### Installation and Dependencies\\n\\nThe installation process for Uvicorn remains straightforward. Developers can install the library using pip with minimal dependencies. The command to install Uvicorn is:\\n\\n```bash\\npip install uvicorn\\n```\\n\\nFor those who prefer to install Uvicorn with 'Cython-based' dependencies and other 'optional extras,' the installation command is:\\n\\n```bash\\npip install uvicorn[standard]\\n```\\n\\nThis flexibility allows developers to choose the installation method that best suits their needs, whether they require a minimal setup or a more feature-rich environment.\\n\\n### Documentation and Resources\\n\\nComprehensive documentation for Uvicorn is available on its official website ([Uvicorn Documentation](https://www.uvicorn.org)). The documentation provides detailed guides on installation, configuration, and usage, making it easier for developers to get started and make the most of the library's features.\\n\\n## Historical Context and Evolution\\n\\nUvicorn has undergone significant development since its initial release. The library was first introduced in April 2018, with version 0.0.1. Over the years, it has seen numerous updates, each bringing new features, performance improvements, and bug fixes. The release history of Uvicorn highlights its rapid evolution and the active development community behind it.\\n\\n### Early Releases\\n\\n- **Version 0.0.1** (April 9, 2018): The initial release of Uvicorn, laying the foundation for its future development.\\n- **Version 0.1.0** (April 20, 2018): Early improvements and bug fixes.\\n- **Version 0.2.0** (June 29, 2018): Introduction of new features and performance enhancements.\\n\\n### Recent Releases\\n\\n- **Version 0.28.0** (January 15, 2024): Significant updates and optimizations.\\n- **Version 0.29.0** (March 20, 2024): The latest release, focusing on performance and compatibility.\\n\\n## Community and Contributions\\n\\nUvicorn is developed and maintained by a dedicated community of developers, led by Tom Christie. The project is hosted on GitHub ([Uvicorn GitHub Repository](https://github.com/encode/uvicorn)), where developers can contribute to its development, report issues, and suggest improvements. The community's active involvement ensures that Uvicorn continues to evolve and meet the needs of modern web development.\\n\\n### Usage and Adoption\\n\\nUvicorn is widely used in the Python web development community, with over 360,000 users and numerous contributors. Its adoption is driven by its performance, ease of use, and compatibility with popular frameworks like FastAPI and Starlette. The library's ability to handle high-concurrency workloads makes it a preferred choice for developers building scalable web applications.\\n\\n## Conclusion\\n\\nThe latest released version of the Uvicorn library, version 0.29.0, represents a significant milestone in its development. With its focus on performance, compatibility, and ease of installation, Uvicorn continues to be a top choice for developers looking to build high-performance web applications in Python. The active community and ongoing contributions ensure that Uvicorn will remain at the forefront of ASGI server implementations, providing developers with the tools they need to succeed.\\n\\n## References\\n\\n- [Uvicorn on PyPI](https://pypi.org/project/uvicorn/)\\n- [Uvicorn Documentation](https://www.uvicorn.org)\\n- [Uvicorn GitHub Repository](https://github.com/encode/uvicorn)\\n\\nThis report provides a detailed overview of the latest released version of the Uvicorn library, highlighting its key features, historical context, and community involvement. The information presented is based on the latest available data as of May 27, 2024."
		},
		{
			"package": "next",
			"current_version": "0.13.0",
			"update": "# Detailed Report on the Latest Released Version of the ```next``` Library\\n\\n## Introduction\\n\\nThe ```next``` library, commonly known as Next.js, is a popular React framework that enables developers to build full-stack web applications with ease. It extends the latest React features and integrates powerful Rust-based JavaScript tooling for the fastest builds. This report aims to provide a comprehensive overview of the latest released version of the Next.js library, focusing on its features, improvements, and significance in the web development community.\\n\\n## Latest Released Version\\n\\nAs of the current date, May 27, 2024, the latest released version of the Next.js library is **14.2.3**. This version was published 16 hours ago, indicating that it is the most recent update available to developers. The release of version 14.2.3 continues to build on the robust foundation of Next.js, offering new features, performance improvements, and bug fixes.\\n\\n## Key Features and Improvements in Version 14.2.3\\n\\n### Performance Enhancements\\n\\nOne of the primary focuses of the Next.js team has always been performance. Version 14.2.3 introduces several optimizations that enhance the speed and efficiency of web applications built with Next.js. These improvements include:\\n\\n- **Faster Build Times**: Leveraging Rust-based JavaScript tooling, the build times have been significantly reduced, allowing developers to iterate more quickly.\\n- **Optimized Server-Side Rendering (SSR)**: Enhancements in SSR ensure that pages load faster, providing a better user experience.\\n- **Improved Static Site Generation (SSG)**: The static site generation process has been optimized to handle larger datasets more efficiently.\\n\\n### New Features\\n\\nVersion 14.2.3 also brings new features that expand the capabilities of Next.js:\\n\\n- **Enhanced Image Optimization**: The image optimization feature now supports more formats and provides better compression, resulting in faster load times and reduced bandwidth usage.\\n- **Middleware Support**: This version introduces middleware support, allowing developers to run code before a request is completed. This is useful for tasks such as authentication, logging, and more.\\n- **Improved API Routes**: API routes have been enhanced to support more use cases and provide better performance.\\n\\n### Bug Fixes\\n\\nIn addition to new features and performance enhancements, version 14.2.3 addresses several bugs reported by the community. These fixes improve the stability and reliability of the framework, ensuring a smoother development experience.\\n\\n## Significance of the Latest Release\\n\\nThe release of Next.js 14.2.3 is significant for several reasons:\\n\\n### Community Impact\\n\\nNext.js is widely used by some of the world's largest companies, and the continuous improvements in the framework directly benefit these organizations. The latest release ensures that developers have access to the most advanced tools and features, enabling them to build high-performance web applications.\\n\\n### Developer Experience\\n\\nThe enhancements in build times and performance optimizations directly impact the developer experience. Faster builds mean that developers can iterate more quickly, leading to increased productivity and shorter development cycles.\\n\\n### Competitive Edge\\n\\nBy continuously improving and adding new features, Next.js maintains its position as a leading framework in the web development ecosystem. The introduction of middleware support and enhanced image optimization keeps Next.js ahead of its competitors, offering developers more flexibility and power.\\n\\n## Conclusion\\n\\nThe latest released version of the Next.js library, version 14.2.3, represents a significant milestone in the framework's development. With its focus on performance enhancements, new features, and bug fixes, this release continues to solidify Next.js as a top choice for developers building full-stack web applications. The improvements in build times, server-side rendering, and static site generation ensure that applications built with Next.js are faster and more efficient than ever before.\\n\\nAs the web development landscape continues to evolve, Next.js remains at the forefront, providing developers with the tools they need to create high-performance, scalable web applications. The release of version 14.2.3 is a testament to the ongoing commitment of the Next.js team to deliver the best possible experience for developers and users alike.\\n\\n## References\\n\\n- [NPMJS - Next](https://www.npmjs.com/package/next)\\n- [GitHub - Vercel/Next.js](https://github.com/vercel/next.js)\\n- [Next.js Documentation](https://nextjs.org/docs)\\n\\nBy providing a detailed and comprehensive overview of the latest released version of the Next.js library, this report aims to inform and assist developers in understanding the significance of the new features and improvements introduced in version 14.2.3."
		}
	],
	"drafted": null
}
"""


def get_cached_data():
    # Parse the JSON string into a Python dictionary

    cached_data = json.loads(data)
    # print(cached_data)
    return cached_data


@app.step
def discover_dependencies(state: dict) -> Any:
    result = steps.discover_dependencies()
    return {
        **state,
        "dependencies": result
    }


@app.step
def research_updates(state: dict) -> Any:
    result = [asyncio.run(steps.research_update(dependency)) for dependency in state['dependencies']]
    return {
        **state,
        "researched": result
    }


@app.step
def draft_issues(state: dict) -> Any:
    # result = steps.run_draft_issues(state['researched'])
    cached_state = get_cached_data()
    result = steps.run_draft_issues(cached_state['researched'])
    return {
        **state,
        "drafted": result
    }


@app.step
def submit_issues(state: dict) -> Any:
    print('decorated submit_issues', state)
    # steps.submit_issues(state['drafted'])
    json_data = json.loads("""
    [
        {
            "package": "uvicorn",
            "title": "Update `uvicorn` Dependency to Version 0.29.0",
            "description": "### Update `uvicorn` Dependency to Version 0.29.0\\n\\n**Current Version:** 0.18.0\\n**New Version:** 0.29.0\\n\\n#### Update Command:\\nTo update `uvicorn` to the latest version, run:\\n```bash\\npip install uvicorn --upgrade\\n```\\n\\n#### Reasons for Update:\\n1. **Performance Enhancements:** Uvicorn 0.29.0 includes several under-the-hood improvements that reduce latency and increase throughput, making it even more suitable for high-traffic applications.\\n2. **Compatibility and Support:** Maintains support for HTTP/1.1 and WebSockets and continues to support Python versions 3.8 and above, aligning with the latest Python developments.\\n3. **Community and Contributions:** Active involvement from the community ensures continuous evolution and development of new features and bug fixes.\\n4. **Additional Options:** For those requiring 'Cython-based' dependencies and other 'optional extras,' the updated installation command offers flexibility with `pip install uvicorn[standard]`.\\n\\n**Label:** `dependency-update`",
            "label": "dependency-update"
        },
        {
            "package": "next",
            "title": "Update `next` Dependency to Version 14.2.3",
            "description": "### Update `next` Dependency to Version 14.2.3\\n\\n**Current Version:** 0.13.0\\n**New Version:** 14.2.3\\n\\n#### Update Command:\\nTo update `next` to the latest version, run:\\n```bash\\nnpm install next@14.2.3\\n```\\n\\n#### Reasons for Update:\\n1. **Performance Enhancements:** Version 14.2.3 introduces faster build times, optimized server-side rendering (SSR), and improved static site generation (SSG).\\n2. **New Features:** Enhanced image optimization, introduction of middleware support, and improved API routes.\\n3. **Bug Fixes:** Addresses several community-reported bugs, improving the stability and reliability of the framework.\\n4. **Community Impact:** Ensures developers have access to the most advanced tools and features, directly benefiting organizations using Next.js.\\n5. **Developer Experience:** Faster builds lead to increased productivity and shorter development cycles.\\n6. **Competitive Edge:** Keeps Next.js ahead of its competitors with continuous improvements and new feature introductions.\\n\\n**Label:** `dependency-update`",
            "label": "dependency-update"
        }
    ]
    """)
    steps.submit_issues(json_data)
    return state
