import { NextRequest, NextResponse } from "next/server";

// Reference to mock tunnels database (would be replaced with real DB in production)
// For simplicity, we're accessing the mock data directly
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

// GET a single tunnel by ID
export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const tunnel = tunnels.find((t) => t.id === params.id);

  if (!tunnel) {
    return NextResponse.json(
      {
        success: false,
        message: "Tunnel not found",
      },
      { status: 404 }
    );
  }

  return NextResponse.json({
    success: true,
    data: tunnel,
  });
}

// DELETE a tunnel by ID
export async function DELETE(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const tunnelIndex = tunnels.findIndex((t) => t.id === params.id);

  if (tunnelIndex === -1) {
    return NextResponse.json(
      {
        success: false,
        message: "Tunnel not found",
      },
      { status: 404 }
    );
  }

  // In a real app, this would delete from database
  tunnels.splice(tunnelIndex, 1);

  return NextResponse.json({
    success: true,
    message: "Tunnel deleted successfully",
  });
}
