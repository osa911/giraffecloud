"use client";

import { useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MoreHorizontal, Plus, Pencil, Trash, ExternalLink } from "lucide-react";
import { format } from "date-fns";
import TunnelDialog from "./TunnelDialog";
import { useTunnels } from "@/hooks/useTunnels";
import type { Tunnel } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";
import { toast } from "@/lib/toast";
import Link from "next/link";

export default function TunnelList() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedTunnel, setSelectedTunnel] = useState<Tunnel | null>(null);
  const { tunnels, isLoading, mutate } = useTunnels();

  const handleOpenDialog = (tunnel?: Tunnel) => {
    setSelectedTunnel(tunnel || null);
    setIsDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setIsDialogOpen(false);
    setSelectedTunnel(null);
  };

  const handleToggleActive = async (tunnel: Tunnel) => {
    try {
      // Optimistic update could be done here, but for simplicity we wait
      await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, {
        is_enabled: !tunnel.is_enabled,
      });
      mutate(); // Refresh the tunnels list
      toast.success(`Tunnel ${!tunnel.is_enabled ? "enabled" : "disabled"}`);
    } catch (error) {
      console.error("Error updating tunnel status:", error);
      toast.error("Failed to update tunnel status");
    }
  };

  const handleDelete = async (tunnel: Tunnel) => {
    if (!confirm("Are you sure you want to delete this tunnel?")) return;

    try {
      await clientApi().delete<void>(`/tunnels/${tunnel.id}`);
      mutate(); // Refresh the tunnels list
      toast.success("Tunnel deleted successfully");
    } catch (error) {
      console.error("Error deleting tunnel:", error);
      toast.error("Failed to delete tunnel");
    }
  };

  if (isLoading) {
    return <div className="p-8 text-center">Loading tunnels...</div>;
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle className="text-xl font-bold">Tunnels</CardTitle>
          <Button onClick={() => handleOpenDialog()}>
            <Plus className="mr-2 h-4 w-4" /> Create Tunnel
          </Button>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  <TableHead>Target Port</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tunnels?.map((tunnel) => (
                  <TableRow key={tunnel.id}>
                    <TableCell className="font-medium">
                      <Link
                        href={`https://${tunnel.domain}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center hover:underline text-primary"
                      >
                        {tunnel.domain}
                        <ExternalLink className="ml-1 h-3 w-3" />
                      </Link>
                    </TableCell>
                    <TableCell>{tunnel.target_port}</TableCell>
                    <TableCell>
                      <Switch
                        checked={tunnel.is_enabled}
                        onCheckedChange={() => handleToggleActive(tunnel)}
                      />
                    </TableCell>
                    <TableCell>{format(new Date(tunnel.created_at), "PPp")}</TableCell>
                    <TableCell>{format(new Date(tunnel.updated_at), "PPp")}</TableCell>
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" className="h-8 w-8 p-0">
                            <span className="sr-only">Open menu</span>
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuLabel>Actions</DropdownMenuLabel>
                          <DropdownMenuItem onClick={() => navigator.clipboard.writeText(`https://${tunnel.domain}`)}>
                            Copy URL
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={() => handleOpenDialog(tunnel)}>
                            <Pencil className="mr-2 h-4 w-4" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => handleDelete(tunnel)} className="text-destructive focus:text-destructive">
                            <Trash className="mr-2 h-4 w-4" /> Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
                {(!tunnels || tunnels.length === 0) && (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center">
                      No tunnels found. Create your first tunnel to get started.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <TunnelDialog
        open={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        tunnel={selectedTunnel}
        onSuccess={() => {
          mutate();
        }}
        existingTunnels={tunnels}
      />
    </>
  );
}
