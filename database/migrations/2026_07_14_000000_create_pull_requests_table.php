<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('pull_requests', function (Blueprint $table) {
            $table->id();

            // GitHub-owned mirror (ADR-0003/0009). Keyed on the GitHub PR id.
            $table->unsignedBigInteger('github_id')->unique();
            $table->integer('number');
            $table->string('repo'); // owner/name
            $table->string('title');
            $table->string('url');
            $table->string('head_ref')->nullable();
            $table->string('state');
            $table->string('suggested_key')->nullable(); // recomputed each sync
            $table->timestamp('synced_at')->nullable();

            // Fylla-owned resolution (ADR-0009), never written by sync.
            $table->unsignedBigInteger('kendo_issue_id')->nullable();
            $table->unsignedBigInteger('kendo_project_id')->nullable();
            $table->string('kendo_key')->nullable();
            $table->timestamp('resolved_at')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('pull_requests');
    }
};
