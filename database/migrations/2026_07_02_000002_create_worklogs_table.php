<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('worklogs', function (Blueprint $table) {
            $table->id();
            $table->foreignId('issue_id')->constrained();
            $table->foreignId('timer_id')->constrained();
            $table->integer('minutes');
            $table->timestamp('started_at');
            $table->text('comment')->nullable();

            // Posting to Kendo is unwired in #9 (model-only, ADR-0001/0003). Reserved.
            $table->timestamp('posted_at')->nullable();
            $table->string('kendo_worklog_id')->nullable();
            $table->text('post_error')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('worklogs');
    }
};
