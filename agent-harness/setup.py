from setuptools import setup, find_packages

setup(
    name="cli-anything-youdaonotelm",
    version="3.0.0",
    description="CLI wrapper for YoudaoNoteLM - RAG-based Youdao Cloud Notes Knowledge Q&A System",
    author="YoudaoNoteLM Team",
    packages=find_packages(),
    entry_points={
        "console_scripts": [
            "youdaonotelm=cli_anything.youdaonotelm:main",
        ],
    },
    python_requires=">=3.10",
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
)
