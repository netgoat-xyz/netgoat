import * as React from "react";

export interface IconProxiedProps extends React.SVGProps<SVGSVGElement> {
  size?: number | string;
  strokeWidth?: number | string;
}

export default function IconProxied({
  className = "",
  size = 24,
  strokeWidth = 2,
  ...props
}: IconProxiedProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={`icon-proxied ${className}`}
      {...props}
    >
      {/* Modern, smooth right arrow */}
      <path
        d="M15 8l4 4-4 4M19 12H5"
        stroke="currentColor"
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* Cloud path remains as scaled */}
      <path
        d="M7.243 18C5.77 18 4 15.993 4 13.517C4 11.042 5.77 9.035 7.243 9.035C7.636 7.273 9.037 5.835 10.918 5.262C12.798 4.69 14.874 5.069 16.362 6.262C17.85 7.452 18.524 9.269 18.132 11.031H19.122C21.035 11.031 22.586 12.591 22.586 14.517C22.586 16.444 21.035 18.004 19.121 18.004H7.243"
        stroke="currentColor"
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
