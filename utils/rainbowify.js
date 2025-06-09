import chalk from "chalk";
const colors = [
  chalk.red,
  chalk.yellow,
  chalk.green,
  chalk.cyan,
  chalk.blue,
  chalk.magenta,
];

const rainbowify = (str) => {
  let i = 0;
  return str
    .split('')
    .map(char => {
      if (char === ' ') return char;
      const colorFn = colors[i % colors.length];
      i++;
      return colorFn(char);
    })
    .join('');
};
export default rainbowify;