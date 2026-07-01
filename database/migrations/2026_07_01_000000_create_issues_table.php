<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('issues', function (Blueprint $table) {
            $table->id();

            // Kendo-mirror fields (owned upstream, overwritten every sync).
            $table->unsignedBigInteger('kendo_id')->unique();
            $table->string('key');
            $table->string('title');
            $table->string('priority')->nullable();
            $table->string('type')->nullable();
            $table->unsignedBigInteger('lane_id')->nullable();
            $table->unsignedBigInteger('project_id')->nullable();
            $table->unsignedBigInteger('sprint_id')->nullable();
            $table->unsignedBigInteger('epic_id')->nullable();
            $table->timestamp('updated_at')->nullable();
            $table->timestamp('synced_at')->nullable();

            // Fylla-owned fields (ADR-0004): reserved, never written by sync.
            $table->date('due_date')->nullable();
            $table->date('not_before')->nullable();
            $table->boolean('up_next')->nullable();
            $table->boolean('no_split')->nullable();
            $table->string('recurrence')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('issues');
    }
};
