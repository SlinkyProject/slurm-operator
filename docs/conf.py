# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information

project = 'slurm-operator'
copyright = 'SchedMD'
author = 'SchedMD'

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = ["myst_parser", "sphinx_design", "sphinx_copybutton", "sphinxmermaid"]
myst_enable_extensions = ["colon_fence"]
myst_fence_as_directive = ["mermaid"]

templates_path = ['_templates']
exclude_patterns = []

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = "sphinx_rtd_theme"
html_logo = "_static/images/slinky.svg"
html_show_sourcelink = False
html_static_path = ['_static']
html_theme_options = {
    "show_nav_level": 3,
    "navigation_depth": 4
}

# Issue https://github.com/executablebooks/MyST-Parser/issues/845
# GitHub admonitions with Sphinx/MyST
# Workaround template adapted from (with some changes):
# https://github.com/python-project-templates/yardang/blob/f77348d45dcf0eb130af304f79c0bfb92ab90e0c/yardang/conf.py.j2#L156-L188


_GITHUB_ADMONITIONS = {
    "> [!NOTE]": "note",
    "> [!TIP]": "tip",
    "> [!IMPORTANT]": "important",
    "> [!WARNING]": "warning",
    "> [!CAUTION]": "caution",
}

def run_convert_github_admonitions_to_rst(app, filename, lines):
    # loop through lines, replace github admonitions
    for i, orig_line in enumerate(lines):
        orig_line_splits = orig_line.split("\n")
        replacing = False
        for j, line in enumerate(orig_line_splits):
            # look for admonition key
            for admonition_key in _GITHUB_ADMONITIONS:
                if admonition_key in line:
                    line = line.replace(admonition_key, ":::{" + _GITHUB_ADMONITIONS[admonition_key] + "}\n")
                    # start replacing quotes in subsequent lines
                    replacing = True
                    break
            else:
                # replace indent to match directive
                if replacing and "> " in line:
                    line = line.replace("> ", "  ")
                elif replacing:
                    # missing "> ", so stop replacing and terminate directive
                    line = f"\n:::\n{line}"
                    replacing = False
            # swap line back in splits
            orig_line_splits[j] = line
        # swap line back in original
        lines[i] = "\n".join(orig_line_splits)

def setup(app):
    app.connect("source-read", run_convert_github_admonitions_to_rst)
