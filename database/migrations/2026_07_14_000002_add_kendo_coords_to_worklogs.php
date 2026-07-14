<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('worklogs', function (Blueprint $table) {
            // Kendo coordinates stamped at roll-up (ADR-0009); PostWorklog posts
            // to these instead of dereferencing ->issue.
            $table->unsignedBigInteger('kendo_project_id')->nullable();
            $table->unsignedBigInteger('kendo_issue_id')->nullable();
        });

        // issue_id survives as nullable provenance — PR worklogs have no local issue.
        Schema::table('worklogs', function (Blueprint $table) {
            $table->foreignId('issue_id')->nullable()->change();
        });
    }

    public function down(): void
    {
        Schema::table('worklogs', function (Blueprint $table) {
            $table->dropColumn(['kendo_project_id', 'kendo_issue_id']);
        });
    }
};
