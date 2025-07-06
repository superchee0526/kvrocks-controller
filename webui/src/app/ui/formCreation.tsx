/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

"use client";
import React from "react";
import FormDialog from "./formDialog";
import {
    createCluster,
    createNamespace,
    createNode,
    createShard,
    importCluster,
    migrateSlot,
} from "../lib/api";
import { useRouter } from "next/navigation";

type NamespaceFormProps = {
    position: string;
    children?: React.ReactNode;
};

type ClusterFormProps = {
    position: string;
    namespace: string;
    children?: React.ReactNode;
};

type ShardFormProps = {
    position: string;
    namespace: string;
    cluster: string;
};

type NodeFormProps = {
    position: string;
    namespace: string;
    cluster: string;
    shard: string;
};

const containsWhitespace = (value: string): boolean => /\s/.test(value);

const validateFormData = (formData: FormData, fields: string[]): string | null => {
    for (const field of fields) {
        const value = formData.get(field);
        if (typeof value === "string" && containsWhitespace(value)) {
            return `${
                field.charAt(0).toUpperCase() + field.slice(1)
            } cannot contain any whitespace characters.`;
        }
    }
    return null;
};

export const NamespaceCreation: React.FC<NamespaceFormProps> = ({ position, children }) => {
    const router = useRouter();
    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["name"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }

        const formObj = Object.fromEntries(formData.entries());
        const response = await createNamespace(formObj["name"] as string);
        if (response === "") {
            router.push(`/namespaces/${formObj["name"]}`);
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Create Namespace"
            submitButtonLabel="Create"
            formFields={[{ name: "name", label: "Input Name", type: "text", required: true }]}
            onSubmit={handleSubmit}
        />
    );
};

export const ClusterCreation: React.FC<ClusterFormProps> = ({ position, namespace, children }) => {
    const router = useRouter();
    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["name", "replicas"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }
        const formObj = Object.fromEntries(formData.entries());
        const nodes = JSON.parse(formObj["nodes"] as string) as string[];
        if (nodes.length === 0) {
            return "Nodes cannot be empty.";
        }

        for (const node of nodes) {
            if (containsWhitespace(node)) {
                return "Nodes cannot contain any whitespace characters.";
            }
        }

        const response = await createCluster(
            formObj["name"] as string,
            nodes,
            parseInt(formObj["replicas"] as string),
            formObj["password"] as string,
            namespace
        );
        if (response === "") {
            router.push(`/namespaces/${namespace}/clusters/${formObj["name"]}`);
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Create Cluster"
            submitButtonLabel="Create"
            formFields={[
                { name: "name", label: "Input Name", type: "text", required: true },
                { name: "nodes", label: "Input Nodes", type: "array", required: true },
                {
                    name: "replicas",
                    label: "Input Replicas",
                    type: "text",
                    required: true,
                },
                {
                    name: "password",
                    label: "Input Password",
                    type: "password",
                },
            ]}
            onSubmit={handleSubmit}
        >
            {children}
        </FormDialog>
    );
};

export const ShardCreation: React.FC<ShardFormProps> = ({ position, namespace, cluster }) => {
    const router = useRouter();
    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["nodes"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }

        const formObj = Object.fromEntries(formData.entries());
        const nodes = JSON.parse(formObj["nodes"] as string) as string[];
        const password = formObj["password"] as string;

        if (nodes.length === 0) {
            return "Nodes cannot be empty.";
        }

        for (const node of nodes) {
            if (containsWhitespace(node)) {
                return "Nodes cannot contain any whitespace characters.";
            }
        }

        const response = await createShard(namespace, cluster, nodes, password);
        if (response === "") {
            router.push(`/namespaces/${namespace}/clusters/${cluster}`);
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Create Shard"
            submitButtonLabel="Create"
            formFields={[
                { name: "nodes", label: "Input Nodes", type: "array", required: true },
                {
                    name: "password",
                    label: "Input Password",
                    type: "password",
                },
            ]}
            onSubmit={handleSubmit}
        />
    );
};

export const ImportCluster: React.FC<ClusterFormProps> = ({ position, namespace, children }) => {
    const router = useRouter();
    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["nodes"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }

        const formObj = Object.fromEntries(formData.entries());
        const nodes = JSON.parse(formObj["nodes"] as string) as string[];
        const cluster = formObj["cluster"] as string;
        const password = formObj["password"] as string;

        if (nodes.length === 0) {
            return "Nodes cannot be empty.";
        }

        for (const node of nodes) {
            if (containsWhitespace(node)) {
                return "Nodes cannot contain any whitespace characters.";
            }
        }

        const response = await importCluster(namespace, cluster, nodes, password);
        if (response === "") {
            router.push(`/namespaces/${namespace}/clusters/${cluster}`);
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Import Cluster"
            submitButtonLabel="Import"
            formFields={[
                {
                    name: "cluster",
                    label: "Input Cluster",
                    type: "text",
                    required: true,
                },
                { name: "nodes", label: "Input Nodes", type: "array", required: true },
                {
                    name: "password",
                    label: "Input Password",
                    type: "password",
                },
            ]}
            onSubmit={handleSubmit}
        >
            {children}
        </FormDialog>
    );
};

export const MigrateSlot: React.FC<ShardFormProps> = ({ position, namespace, cluster }) => {
    const router = useRouter();
    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["target", "slot", "slot_only"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }

        const formObj = Object.fromEntries(formData.entries());
        const target = parseInt(formObj["target"] as string);
        const slot = parseInt(formObj["slot"] as string);
        const slotOnly = formObj["slot_only"] === "true";

        const response = await migrateSlot(namespace, cluster, target, slot, slotOnly);
        if (response === "") {
            window.location.reload();
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Migrate Slot"
            submitButtonLabel="Migrate"
            formFields={[
                { name: "target", label: "Input Target", type: "text", required: true },
                { name: "slot", label: "Input Slot", type: "text", required: true },
                {
                    name: "slot_only",
                    label: "Slot Only",
                    type: "enum",
                    values: ["true", "false"],
                    required: true,
                },
            ]}
            onSubmit={handleSubmit}
        />
    );
};

export const NodeCreation: React.FC<NodeFormProps> = ({ position, namespace, cluster, shard }) => {
    const router = useRouter();

    const handleSubmit = async (formData: FormData) => {
        const fieldsToValidate = ["Address", "Role"];
        const errorMessage = validateFormData(formData, fieldsToValidate);
        if (errorMessage) {
            return errorMessage;
        }

        const formObj = Object.fromEntries(formData.entries());
        const address = formObj["Address"] as string;
        const role = formObj["Role"] as string;
        const password = formObj["Password"] as string;

        if (containsWhitespace(address)) {
            return "Address cannot contain any whitespace characters.";
        }

        const response = await createNode(namespace, cluster, shard, address, role, password);
        if (response === "") {
            window.location.reload();
        } else {
            return "Invalid form data";
        }
    };

    return (
        <FormDialog
            position={position}
            title="Create Node"
            submitButtonLabel="Create"
            formFields={[
                {
                    name: "Address",
                    label: "Input Address",
                    type: "text",
                    required: true,
                },
                {
                    name: "Role",
                    label: "Select Role",
                    type: "enum",
                    required: true,
                    values: ["master", "slave"],
                },
                {
                    name: "Password",
                    label: "Input Password",
                    type: "password",
                },
            ]}
            onSubmit={handleSubmit}
        />
    );
};
