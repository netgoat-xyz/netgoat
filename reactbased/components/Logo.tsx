export function Logo(props: React.ComponentPropsWithoutRef<"svg">) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="100"
      height="50"
      fill="none"
      viewBox="0 0 344 145"
      {...props}
    >
      <path
        fill="#919191"
        stroke="#D3D3D3"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="4"
        d="M5.328 49.848a5.78 5.78 0 0 1 0-3.648C13.246 22.38 35.72 5.197 62.206 5.197c26.475 0 48.938 17.165 56.872 40.975.4 1.181.4 2.46 0 3.647-7.912 23.821-30.385 41.003-56.872 41.003-26.475 0-48.943-17.165-56.878-40.974"
      />
      <path
        fill="#919191"
        stroke="#D3D3D3"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="10"
        d="M5.328 49.848a5.78 5.78 0 0 1 0-3.648C13.246 22.38 35.72 5.197 62.206 5.197c26.475 0 48.938 17.165 56.872 40.975.4 1.181.4 2.46 0 3.647-7.912 23.821-30.385 41.003-56.872 41.003-26.475 0-48.943-17.165-56.878-40.974"
      />
      <path
        stroke="#D3D3D3"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="10"
        d="M79.331 48.01a17.125 17.125 0 1 1-34.25 0 17.125 17.125 0 0 1 34.25 0"
      />

      {/* fixed foreignObject */}
      <foreignObject
        width="103.312"
        height="91.563"
        x="10"
        y="57"
        clipPath="url(#bgblur_0_0_1_clip_path)"
      >
        <div style={{ backdropFilter: "blur(2px)", height: "100%", width: "100%" }} />
      </foreignObject>

      <g data-figma-bg-blur-radius="4">
        <path
          fill="#919191"
          fillOpacity="0.76"
          d="M104.313 126.187v-1.016a20.5 20.5 0 0 0-.525-4.592l-9.923-42.978a15.1 15.1 0 0 0-5.205-8.35A14.6 14.6 0 0 0 79.493 66H43.82a14.6 14.6 0 0 0-9.166 3.252 15.1 15.1 0 0 0-5.205 8.349l-9.923 42.978a20.4 20.4 0 0 0-.525 4.592v1.016m85.313 0c0 3.548-1.383 6.95-3.845 9.458a13 13 0 0 1-9.28 3.917H32.124c-3.481 0-6.82-1.409-9.28-3.917A13.5 13.5 0 0 1 19 126.187m85.313 0c0-3.547-1.383-6.949-3.845-9.457a13 13 0 0 0-9.28-3.918H32.124c-3.481 0-6.82 1.41-9.28 3.918A13.5 13.5 0 0 0 19 126.187"
        />
        <path
          stroke="#D3D3D3"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeOpacity="0.76"
          strokeWidth="10"
          d="M104.313 126.187v-1.016a20.5 20.5 0 0 0-.525-4.592l-9.923-42.978a15.1 15.1 0 0 0-5.205-8.35A14.6 14.6 0 0 0 79.493 66H43.82a14.6 14.6 0 0 0-9.166 3.252 15.1 15.1 0 0 0-5.205 8.349l-9.923 42.978a20.4 20.4 0 0 0-.525 4.592v1.016"
        />
      </g>

      {/* second foreignObject */}
      <foreignObject
        width="103.312"
        height="91.563"
        x="10"
        y="57"
        clipPath="url(#bgblur_1_0_1_clip_path)"
      >
        <div style={{ backdropFilter: "blur(2px)", height: "100%", width: "100%" }} />
      </foreignObject>

      <g data-figma-bg-blur-radius="4">
        <path
          fill="#919191"
          fillOpacity="0.76"
          d="M104.313 126.187v-1.016a20.5 20.5 0 0 0-.525-4.592l-9.923-42.978a15.1 15.1 0 0 0-5.205-8.35A14.6 14.6 0 0 0 79.493 66H43.82a14.6 14.6 0 0 0-9.166 3.252 15.1 15.1 0 0 0-5.205 8.349l-9.923 42.978a20.4 20.4 0 0 0-.525 4.592v1.016"
        />
      </g>

      <path
        fill="#D3D3D3"
        d="M160.4 92V58.4h5.088l16.368 19.44..."
      />

      <defs>
        <clipPath id="bgblur_0_0_1_clip_path" transform="translate(-10 -57)">
          <path d="M104.313 126.187v-1.016a20.5..." />
        </clipPath>
        <clipPath id="bgblur_1_0_1_clip_path" transform="translate(-10 -57)">
          <path d="M104.313 126.187v-1.016a20.5..." />
        </clipPath>
      </defs>
    </svg>
  )
}
