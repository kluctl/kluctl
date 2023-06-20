
module.exports = function override(config, env) {
    if (env !== "production") {
        return config;
    }

    // we disable minimize as we're actually committing the build folder into git, so that people who don't want to deal
    // with the UI while working on Go code don't have to use npm and friends
    config.optimization.minimize = false

    // we use "[name].dummy.xxx" to ensure that fix-assets doesn't replace something by mistake

    // Get rid of hash for js files
    config.output.filename = "static/js/[name].dummy.js"
    config.output.chunkFilename = "static/js/[name].dummy.chunk.js"

    // Get rid of hash for media files
    config.output.assetModuleFilename = 'static/media/[name].dummy[ext]'
    config.module.rules[1].oneOf[2].use[1].options.name = 'static/media/[name].dummy.[ext]'

    // Get rid of hash for css files
    const miniCssExtractPlugin = config.plugins.find(element => element.constructor.name === "MiniCssExtractPlugin");
    miniCssExtractPlugin.options.filename = "static/css/[name].dummy.css"
    miniCssExtractPlugin.options.chunkFilename = "static/css/[name].dummy.chunk.css"

    return config;
};