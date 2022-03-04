# Dynamic dependencies example

This is a boilerplate template that shows an example of using dependencies. It specifies both the
[docs](/examples/for-learning-and-testing/docs) and [website](/examples/for-learning-and-testing/website) examples as
dependencies to show how one boilerplate template can pull in another, and that you can use interpolation in the
`template-url` and `output-folder` parameters of dependencies to dynamically specify where to read the template and
where to write the output. It also defines all the variables needed for both of those dependencies at the top level to
show how variable inheritance works.
