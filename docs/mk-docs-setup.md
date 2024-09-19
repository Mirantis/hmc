# HMC MKdocs Setup

## Project layout

    mkdocs.yml    # The configuration file.
    docs/
        index.md  # The documentation homepage.
        stylesheets  # CSS stylesheets to control look and feel
        assets  # Images and other served material
        ...       # Other markdown pages, images and other files.


## Setting up MKdocs and dependancies

1. Setup python Virtual Environment 

    `python3  -m venv ./mkdocs`
    `source ./mkdocs/bin/activate`

2. Install MkDocs

    `pip install mkdocs`

3. Install plugins

    `pip install mkdocs-mermaid2-plugin`

    `pip install mkdocs-material`

## Run MKdocs for dev

* `mkdocs serve` - Start the live-reloading docs server.

For full documentation visit [mkdocs.org](https://www.mkdocs.org).

## MKdocs Commands

* `mkdocs new [dir-name]` - Create a new project.
* `mkdocs serve` - Start the live-reloading docs server.
* `mkdocs build` - Build the documentation site.
* `mkdocs -h` - Print help message and exit.

