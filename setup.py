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

datas = []
for dirpath, dirnames, filenames in os.walk("./kluctl"):
    for f in filenames:
        if f.endswith(".py") or f.endswith(".pyc"):
            continue
        p = os.path.join("..", dirpath, f)
        datas.append(p)

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
    package_data={
       "": datas,
       "kluctl": ["bootstrap/**"],
    },
    install_requires=[
        "click>=8.0.1",
        "click-option-group>=0.5.3",
        "termcolor>=1.1.0",
        "jinja2>=3.0.2",
        "requests>=2.26.0",
        "requests_ntlm>=1.1.0",
        "pyyaml>=6.0b1",
        "deepdiff>=5.5.0",
        "kubernetes>=19.15.0a1",
        "adal>=1.2.7",
        "PyJWT>=2.2.0",
        "python-dxf>=7.7.1",
        "gitpython>=3.1.24",
        "jsonschema>=4.2.1",
        "filelock>=3.3.0",
        "python-gitlab>=2.10.1",
        "jsonpath-ng>=1.5.3",
        "jsonpatch>=1.32",
        "boto3>=1.20.24",
    ],
    entry_points={
        "console_scripts": [
            "kluctl=kluctl.cli.__main__:main"
        ]
    },
)
