import { Link as MuiLink, LinkProps as MuiLinkProps } from "@mui/material";
import NextLink from "next/link";

/**
 * For future updates, see:
 * https://gist.github.com/kachar/028b6994eb6b160e2475c1bb03e33e6a
 */
const Link = (props: MuiLinkProps<"a">) => {
  return <MuiLink component={NextLink} {...props} />;
};

export default Link;
