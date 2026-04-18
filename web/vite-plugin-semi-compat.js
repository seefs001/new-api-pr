import fs from 'fs';
import path from 'path';
import { compileString, Logger } from 'sass';

function injectSemiTheme(source, options = {}) {
  const {
    theme = '@douyinfe/semi-theme-default',
    cssLayer = false,
    prefixCls = 'semi',
  } = options;

  const themeImports = [
    `@import "${theme}/scss/index.scss";`,
  ];

  const isBaseStyle = source.includes('semi-base');
  if (isBaseStyle) {
    themeImports.unshift(`@import "${theme}/scss/global.scss";`);

    const animationFile = path.join(
      'node_modules',
      ...theme.split('/'),
      'scss',
      'animation.scss',
    );
    if (fs.existsSync(animationFile)) {
      themeImports.unshift(`@import "${theme}/scss/animation.scss";`);
    }
  }

  let finalSource = `${themeImports.join('\n')}\n$prefix: '${prefixCls}';\n${source}`;
  finalSource = finalSource.replace(
    /@import\s+(['"])~([^'"]+)\1;/g,
    '@import "$2";',
  );
  if (cssLayer) {
    finalSource = `@layer semi {\n${finalSource}\n}`;
  }
  return finalSource;
}

export default function semiViteCompatPlugin(options = {}) {
  return {
    name: 'vite-plugin-semi-compat',
    load(id) {
      if (!/@douyinfe\/semi-(ui|icons|foundation)\/lib\/.+\.css$/.test(id)) {
        return null;
      }

      const scssFilePath = id.replace(/\.css$/, '.scss');
      if (!fs.existsSync(scssFilePath)) {
        return null;
      }

      const scssSource = fs.readFileSync(scssFilePath, 'utf8');
      const themedScss = injectSemiTheme(scssSource, options);
      const css = compileString(themedScss, {
        importers: [],
        loadPaths: [
          path.dirname(scssFilePath),
          path.resolve(process.cwd(), 'node_modules'),
        ],
        logger: Logger.silent,
      }).css;

      return css;
    },
  };
}
