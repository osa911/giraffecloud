import { NextRequest, NextResponse } from "next/server";

// Mock database for development
const tunnels = [
  {
    id: "1",
    name: "Local Web Server",
    localPort: 8080,
    remotePort: 42568,
    protocol: "http",
    publicUrl: "https://local-web-xyz.giraffecloud.dev",
    status: "online",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: "2",
    name: "API Server",
    localPort: 3000,
    remotePort: 23456,
    protocol: "http",
    publicUrl: "https://api-xyz.giraffecloud.dev",
    status: "offline",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
];

export async function GET() {
  // Simulate API delay
  await new Promise((resolve) => setTimeout(resolve, 500));

  return NextResponse.json({
    success: true,
    data: tunnels,
  });
}

export async function POST(request: NextRequest) {
  const body = await request.json();

  // Validate request
  if (!body.name || !body.localPort || !body.protocol) {
    return NextResponse.json(
      {
        success: false,
        message: "Missing required fields",
      },
      { status: 400 }
    );
  }

  // Create new tunnel
  const newTunnel = {
    id: Math.random().toString(36).substring(7),
    name: body.name,
    localPort: body.localPort,
    remotePort: Math.floor(Math.random() * 20000) + 10000,
    protocol: body.protocol,
    publicUrl: `https://${body.name
      .toLowerCase()
      .replace(/\s+/g, "-")}-xyz.giraffecloud.dev`,
    status: "offline",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  tunnels.push(newTunnel);

  return NextResponse.json(
    {
      success: true,
      data: newTunnel,
    },
    { status: 201 }
  );
}
