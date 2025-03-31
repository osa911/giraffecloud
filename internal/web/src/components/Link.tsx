import { forwardRef } from "react";
import { useRouter } from "next/navigation";
import { Link as MuiLink, LinkProps as MuiLinkProps } from "@mui/material";

export interface LinkProps extends Omit<MuiLinkProps, "href"> {
  href: string;
  scroll?: boolean;
}

const Link = forwardRef<HTMLAnchorElement, LinkProps>(
  ({ href, children, scroll = true, ...props }, ref) => {
    const router = useRouter();

    return (
      <MuiLink
        ref={ref}
        href={href}
        onClick={(e) => {
          e.preventDefault();
          router.push(href, { scroll });
        }}
        {...props}
      >
        {children}
      </MuiLink>
    );
  }
);

Link.displayName = "Link";

export default Link;
