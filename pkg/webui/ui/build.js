const rewire = require('rewire');
const defaults = rewire('react-scripts/scripts/build.js');
const config = defaults.__get__('config');

// we disable minimize as we're actually committing the build folder into git, so that people who don't want to deal
// with the UI while working on Go code don't have to use npm and friends
config.optimization.minimize = false
