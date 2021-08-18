"""Setup script for kluctl"""

import os

from setuptools import setup, find_packages

# The directory containing this file
HERE = os.path.abspath(os.path.dirname(__file__))

name = 'kluctl'
filename = '{0}/_version.py'.format(name)
_locals = {}
with open(filename) as src:
    exec(src.read(), None, _locals)
version = _locals['__version__']

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

# This call to setup() does all the work
setup(
    name=name,
    version=version,
    description="Deploy and manage complex deployments on Kubernetes",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/codablock/kluctl",
    author="Alexander Block",
    author_email="ablock@codablock.de",
    license="Apache",
    classifiers=[
        "License :: OSI Approved :: Apache Software License",
        "Intended Audience :: Developers",
        "Intended Audience :: Information Technology",
        "Intended Audience :: System Administrators",
        "Operating System :: MacOS :: MacOS X",
        "Operating System :: POSIX",
        "Operating System :: Unix",
        "Programming Language :: Python",
        "Programming Language :: Python :: 3",
    ],
    packages=find_packages(),
    include_package_data=True,
    install_requires=[
        "click>=8.0.1",
        "click-option-group>=0.5.3",
        "termcolor>=1.1.0",
        "jinja2>=3.0.1",
        "requests>=2.26.0",
        "requests_ntlm>=1.1.0",
        "pyyaml>=5.4.1",
        "deepdiff>=5.5.0",
        "kubernetes>=18.20.0b1",
        "adal>=1.2.7",
        "PyJWT>=2.1.0",
        "python-dxf>=7.7.1",
        "gitpython>=3.1.18",
        "jsonschema>=3.2.0",
        "filelock==3.0.12",
        "python-gitlab==2.10.0",
    ],
    entry_points={
        "console_scripts": [
            "kluctl=kluctl.cli.__main__:main"
        ]
    },
)
